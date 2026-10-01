package dos

import (
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Критические ошибки (INT 24h) и Ctrl-C/Ctrl-Break (INT 23h, INT 1Bh).
//
// Источники: RBIL (Int 23, Int 24, Int 1B, Int 09), исходники MS-DOS 4.0
// (github.com/microsoft/MS-DOS v4.0, MIT): DOS/CTRLC.ASM (CNTCHAND, HardErr,
// FATAL: разбор ответа обработчика), BIOS/MSCON.ASM (CBREAK, CON$RDND).
//
// Обработчик пользователя вызывается как гостевой код: HLE-обработчик INT 21h
// кладёт кадр INT на стек, ставит CS:IP на заглушку в ROM («trap; IRET») и
// переходит по вектору. Когда обработчик возвращается, срабатывает ловушка
// заглушки (critCont/ctrlCCont), и вызов INT 21h продолжается или
// завершается; следующий за ловушкой IRET возвращает управление программе.

// Ошибки устройства, как в INT 24h (RBIL, табл. 02545).
const (
	critWriteProtect = 0x00
	critNotReady     = 0x02
)

// errFailI24 — расширенная ошибка «Fail on INT 24» (INT 21h/59h).
const errFailI24 = 0x53

// offBlockDev — заголовок блочного устройства для BP:SI обработчика INT 24h.
const offBlockDev = 0x02C0

// critKind — как вызов INT 21h возвращает ошибку, если обработчик ответил Fail.
type critKind byte

const (
	kindPath critKind = iota // файловые вызовы с путём: CF=1, AX=3
	kindFree                 // 36h: AX=FFFFh
)

// critState — что нужно помнить, пока выполняется обработчик INT 24h.
type critState struct {
	sp   uint16 // SP на входе в обработчик INT 21h
	kind critKind
}

// ccState — то же для обработчика INT 23h.
type ccState struct {
	ax uint16
	sp uint16
}

// driveFault — состояние накопителя, при котором DOS вызывает INT 24h.
type driveFault struct {
	notReady     bool
	writeProtect bool
}

// faultCode возвращает код ошибки INT 24h для операции с диском drive.
func (d *DOS) faultCode(drive int, write bool) (byte, bool) {
	f := d.faults[drive]
	switch {
	case f.notReady:
		return critNotReady, true
	case f.writeProtect && write:
		return critWriteProtect, true
	}
	return 0, false
}

// pathArg возвращает указатель на путь вызова INT 21h и признаки операции:
// write — вызов пишет на диск, ok — вызов обращается к диску по пути.
func (d *DOS) pathArg(e *hle.Env) (p []byte, write, ok bool) {
	c := e.CPU
	ah, al := c.AH(), c.AL()
	if ah == 0x71 {
		if d.noLFN {
			return nil, false, false
		}
		switch al {
		case 0x39, 0x3A, 0x41, 0x56:
			write = true
		case 0x43:
			write = c.BL()&1 == 1 // установка атрибутов или времени
		case 0x3B, 0x4E:
		default:
			return nil, false, false
		}
		return e.Mem.ASCIIZ(e.DSDX(), maxLFNPath+8), write, true
	}
	switch ah {
	case 0x39, 0x3A, 0x3C, 0x41, 0x56, 0x5B:
		write = true
	case 0x43:
		write = al == 1
	case 0x3B, 0x3D, 0x4E:
	case 0x4B:
		if al != 0 && al != 3 {
			return nil, false, false
		}
	default:
		return nil, false, false
	}
	return e.Mem.ASCIIZ(e.DSDX(), 128), write, true
}

// guard проверяет, не обратился ли вызов INT 21h к накопителю в сбойном
// состоянии. Если да, DOS вызывает INT 24h; возвращает true, если вызов
// приостановлен (продолжится в critCont).
func (d *DOS) guard(e *hle.Env) bool {
	if !d.anyFault {
		return false
	}
	c := e.CPU
	if c.AH() == 0x36 {
		drive := int(c.DL())
		if drive == 0 {
			drive = d.fs.cur + 1
		}
		if drive < 1 || drive > lastDrive || d.fs.drives[drive-1] == "" {
			return false
		}
		if code, bad := d.faultCode(drive-1, false); bad {
			d.critical(e, drive-1, false, 1, code, kindFree)
			return true
		}
		return false
	}
	p, write, ok := d.pathArg(e)
	if !ok {
		return false
	}
	drive, _, errc := d.fs.canon(p, false)
	if errc != 0 {
		return false
	}
	if code, bad := d.faultCode(drive, write); bad {
		d.critical(e, drive, write, 2, code, kindPath)
		return true
	}
	return false
}

// critical вызывает INT 24h (RBIL Int 24). area: 0 — DOS, 1 — FAT, 2 —
// корневой каталог, 3 — данные. Разрешены Retry и Fail; Ignore не разрешён
// (для хоста «продолжить как ни в чём не бывало» после ошибки бессмысленно,
// а RBIL: запрещённый Ignore превращается в Fail).
func (d *DOS) critical(e *hle.Env, drive int, write bool, area, code byte, kind critKind) {
	c := e.CPU
	st := critState{sp: c.R[cpu.SP], kind: kind}
	if d.errorMode {
		// RBIL: ошибка внутри обработчика — вызов тут же завершается Fail.
		e.Note("critical error inside INT 24h: failed")
		d.critFail(e, st)
		return
	}
	d.errorMode = true
	d.crit = st
	// Стек по RBIL: кадр INT 21h, под ним регистры вызова (ES, DS, BP, DI,
	// SI, DX, CX, BX, AX; AX на вершине), затем кадр INT 24h.
	push := func(v uint16) {
		c.R[cpu.SP] -= 2
		c.Write16(cpu.SS, c.R[cpu.SP], v)
	}
	push(e.Seg(cpu.ES))
	push(e.Seg(cpu.DS))
	for _, r := range []int{cpu.BP, cpu.DI, cpu.SI, cpu.DX, cpu.CX, cpu.BX, cpu.AX} {
		push(c.R[r])
	}
	ah := byte(0x18) | area<<1 // разрешены Fail (бит 3) и Retry (бит 4)
	if write {
		ah |= 1
	}
	e.Note("INT 24h: drive %c: error %02Xh, AH=%02X", 'A'+drive, code, ah)
	c.SetAH(ah)
	c.SetAL(byte(drive))
	c.R[cpu.DI] = uint16(code)
	c.R[cpu.BP] = dataSeg
	c.R[cpu.SI] = offBlockDev
	c.SetSeg(cpu.CS, hle.ROMSeg)
	c.IP = d.critStub
	c.SetFlags(c.Flags | cpu.FlagIF)
	c.Interrupt(0x24)
}

// critCont продолжает вызов INT 21h после возврата из обработчика INT 24h.
// Ответ (DOS/CTRLC.ASM, FAILRET): 0 Ignore и 1 Retry — если разрешены; 3 Fail
// — если разрешён, иначе Abort; всё остальное — Abort.
func (d *DOS) critCont(e *hle.Env) error {
	c := e.CPU
	action := c.AL()
	d.errorMode = false
	st := d.crit
	pop := func() uint16 {
		v := c.Read16(cpu.SS, c.R[cpu.SP])
		c.R[cpu.SP] += 2
		return v
	}
	c.R[cpu.AX] = pop()
	c.R[cpu.BX] = pop()
	c.R[cpu.CX] = pop()
	c.R[cpu.DX] = pop()
	c.R[cpu.SI] = pop()
	c.R[cpu.DI] = pop()
	c.R[cpu.BP] = pop()
	c.SetSeg(cpu.DS, pop())
	c.SetSeg(cpu.ES, pop())
	c.R[cpu.SP] = st.sp
	switch action {
	case 1: // Retry
		e.Note("INT 24h answered Retry")
		return d.redispatch(e)
	case 0, 3: // Ignore (не разрешён) и Fail
		e.Note("INT 24h answered %s: the call fails", map[byte]string{0: "Ignore (not allowed)", 3: "Fail"}[action])
		d.critFail(e, st)
	default: // Abort
		e.Note("INT 24h answered Abort: the program ends")
		d.terminateAs(0, exitHardError)
	}
	return nil
}

// critFail завершает вызов INT 21h ошибкой после Fail. Код расширенной
// ошибки — 53h («Fail on INT 24», MS-DOS 4.0 ERROR.INC); AX, который видит
// программа, зависит от вызова: для вызовов с путём это «путь не найден» (так
// MS-DOS 4.0 RENAME.ASM, BAD_PATH, поступает при FAILERR), для 36h — FFFFh.
func (d *DOS) critFail(e *hle.Env, st critState) {
	d.lastErr = errFailI24
	switch st.kind {
	case kindFree:
		e.CPU.R[cpu.AX] = 0xFFFF
	default:
		e.CPU.R[cpu.AX] = errPathNotFound
		e.SetCF(true)
	}
}

// --- Ctrl-C / Ctrl-Break --------------------------------------------------------

// int1B — обработчик Ctrl-Break по умолчанию (MSCON.ASM, CBREAK): помнит
// «нажато ^C», DOS заметит это при следующей проверке консоли.
func (d *DOS) int1B(e *hle.Env) error {
	d.breakChar = true
	return nil
}

// skipNulls выбрасывает слова 0000h из буфера клавиатуры: BIOS кладёт их
// после Ctrl-Break, драйвер CON их пропускает (MSCON.ASM, CHRIN).
func (d *DOS) skipNulls() {
	for {
		w, ok := d.b.PeekKey()
		if !ok || w != 0 {
			return
		}
		d.b.ReadKey()
	}
}

// ctrlCWaiting reports whether the next console character is a ^C.
func (d *DOS) ctrlCWaiting() bool {
	d.skipNulls()
	if d.breakChar {
		return true
	}
	w, ok := d.b.PeekKey()
	return ok && byte(w) == 3
}

// statChk — проверка ^C функциями консоли DOS (CTRLC.ASM, STATCHK): если
// первое «слово» в буфере — ^C (или был Ctrl-Break), ^C съедается и вызывается
// INT 23h. Возвращает true, если вызов приостановлен.
func (d *DOS) statChk(e *hle.Env) bool {
	d.skipNulls()
	if d.breakChar {
		d.breakChar = false
	} else if w, ok := d.b.PeekKey(); ok && byte(w) == 3 {
		d.b.ReadKey()
	} else {
		return false
	}
	d.ctrlC(e)
	return true
}

// ctrlC печатает «^C», CR LF и вызывает INT 23h (CNTCHAND): CLC, регистры
// пользователя, кадр INT 21h на стеке.
func (d *DOS) ctrlC(e *hle.Env) {
	c := e.CPU
	d.conWrite([]byte("^C\r\n"))
	d.cc = append(d.cc, ccState{ax: c.R[cpu.AX], sp: c.R[cpu.SP]})
	e.Note("^C: INT 23h")
	c.SetSeg(cpu.CS, hle.ROMSeg)
	c.IP = d.ccStub
	c.SetFlags(c.Flags&^cpu.FlagCF | cpu.FlagIF)
	c.Interrupt(0x23)
}

// ctrlCCont разбирает возврат из INT 23h (RBIL Int 23, CTRLC.ASM): IRET —
// вызов повторяется; RETF оставляет слово флагов, DOS его снимает, и при CF=1
// программа завершается (тип выхода 1, код 0), иначе вызов повторяется.
func (d *DOS) ctrlCCont(e *hle.Env) error {
	c := e.CPU
	st := d.cc[len(d.cc)-1]
	d.cc = d.cc[:len(d.cc)-1]
	if c.R[cpu.SP] != st.sp {
		abort := c.Flags&cpu.FlagCF != 0
		c.R[cpu.SP] = st.sp
		if abort {
			e.Note("INT 23h returned with CF: the program ends")
			d.terminateAs(0, exitCtrlC)
			return nil
		}
	}
	e.Note("INT 23h returned: the call is repeated")
	c.R[cpu.AX] = st.ax
	return d.redispatch(e)
}

// redispatch повторяет вызов INT 21h из заглушки продолжения. Если вызов должен
// ждать (cpu.ErrRetry), повтор ловушки заглушки был бы неверен, поэтому
// управление передаётся на заглушку самого INT 21h: она повторится сама.
func (d *DOS) redispatch(e *hle.Env) error {
	err := d.int21(e)
	if err == cpu.ErrRetry {
		c := e.CPU
		c.Flags |= cpu.FlagIF
		c.SetSeg(cpu.CS, hle.ROMSeg)
		c.IP = d.int21Off
		return nil
	}
	return err
}

// Типы завершения (INT 21h/4Dh, AH).
const (
	exitCtrlC     = 1
	exitHardError = 2
)

func (d *DOS) installCrit(e *hle.Env) {
	d.critStub = e.IRETStub(e.Register("crit-cont", d.critCont))
	d.ccStub = e.IRETStub(e.Register("ctrlc-cont", d.ctrlCCont))
	// Заголовок блочного устройства для BP:SI (RBIL: бит 15 слова +4 —
	// символьное устройство; у блочного он сброшен).
	m := e.Mem
	a := mem.Lin(dataSeg, offBlockDev)
	m.W32(a, 0xFFFFFFFF)
	m.W16(a+4, 0)
	m.W8(a+10, 1)
	// INT 23h по умолчанию: STC, RETF — «завершить программу».
	e.SetVector(0x23, hle.ROMSeg, e.Emit([]byte{0xF9, 0xCB}))
	e.HookInt(0x1B, "int1B", d.int1B)
}
