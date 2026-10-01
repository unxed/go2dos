package dos

import (
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// Alternate Multiplex Interrupt Specification (AMIS) 3.6, INT 2Dh — общий
// диспетчер для программ эмулятора, которые должны быть видны гостю по
// сигнатуре (UTF-8 имена, TEXTWIN, HOSTEXEC). Источник — первоисточник в RBIL
// («INT 2D - ALTERNATE MULTIPLEX INTERRUPT SPECIFICATION (AMIS) [v3.6]»).
//
// Программа ищет свой провайдер, перебирая AH=00h-FFh с AL=00h, и сравнивает
// первые 16 байт сигнатуры (производитель и продукт по 8 символов). Номер
// выбирает эмулятор, а не фиксированная константа: первый свободный, начиная
// с amisFirstMux.

const amisFirstMux = 0xC0 // решение: редко занятая область; спека номер не задаёт

// AMISProvider — «резидентная программа» эмулятора на INT 2Dh.
type AMISProvider struct {
	Manufacturer string // до 8 символов, добивается пробелами
	Product      string // до 8 символов, добивается пробелами
	Description  string // до 63 символов (ASCIIZ после 16 байт сигнатуры)
	Version      uint16 // CH — старшая, CL — младшая
	// Call обрабатывает функции AL=10h-FFh (частный API). false — функция не
	// реализована (ответ AL=00h).
	Call func(e *hle.Env, fn byte) (bool, error)
}

type amisEntry struct {
	p      AMISProvider
	sigOff uint16 // подпись в ROM (сегмент hle.ROMSeg)
}

func pad8(s string) string {
	b := []byte(s)
	for len(b) < 8 {
		b = append(b, ' ')
	}
	return string(b[:8])
}

// RegisterAMIS регистрирует провайдера и возвращает его мультиплексный номер.
func (d *DOS) RegisterAMIS(p AMISProvider) (byte, error) {
	for mux := amisFirstMux; mux <= 0xFF; mux++ {
		if d.amis[byte(mux)] != nil {
			continue
		}
		desc := p.Description
		if len(desc) > 63 {
			desc = desc[:63]
		}
		sig := append([]byte(pad8(p.Manufacturer)+pad8(p.Product)+desc), 0)
		d.amis[byte(mux)] = &amisEntry{p: p, sigOff: d.e.Emit(sig)}
		return byte(mux), nil
	}
	return 0, errAMISFull
}

var errAMISFull = amisError("no free AMIS multiplex number")

type amisError string

func (e amisError) Error() string { return string(e) }

// installAMIS ставит INT 2Dh: заголовок протокола разделения прерываний IBM
// (ISP, RBIL табл. 02568), список перехваченных прерываний для AL=04h и сам
// обработчик. Сзади в цепочке стоит IRET: других AMIS-программ в эмуляторе нет.
func (d *DOS) installAMIS(e *hle.Env) {
	d.amis = map[byte]*amisEntry{}
	iret := e.Emit([]byte{0xCF})
	seg := uint16(hle.ROMSeg)
	trap := e.Register("int2D", d.int2D)
	hdr := []byte{
		0xEB, 0x10, // короткий переход на обработчик сразу за блоком
		byte(iret), byte(iret >> 8), byte(seg), byte(seg >> 8), // следующий обработчик
		0x4B, 0x42, // подпись 424Bh
		0x00,       // флаг EOI: программное прерывание
		0xEB, 0x0C, // переход на процедуру сброса (RETF) после обработчика
		0, 0, 0, 0, 0, 0, 0, // зарезервировано
	}
	code := append(hle.Trap(trap), 0xCF, 0xCB) // обработчик: ловушка, IRET; сброс: RETF
	off := e.Emit(append(hdr, code...))
	e.SetVector(0x2D, hle.ROMSeg, off)
	// Список для AL=04h: номер прерывания и смещение обработчика; последняя
	// запись — 2Dh.
	d.amisHooks = e.Emit([]byte{0x2D, byte(off), byte(off >> 8)})
}

func (d *DOS) int2D(e *hle.Env) error {
	c := e.CPU
	mux, fn := c.AH(), c.AL()
	ent := d.amis[mux]
	if ent == nil {
		c.SetAL(0) // номер свободен
		return nil
	}
	switch fn {
	case 0x00: // проверка установки
		c.SetAL(0xFF)
		c.R[cpu.CX] = ent.p.Version
		c.R[cpu.DX] = hle.ROMSeg
		c.R[cpu.DI] = ent.sigOff
	case 0x01: // частной точки входа нет: всё через INT 2Dh
		c.SetAL(0x00)
	case 0x02: // выгрузка: эмулятор не выгружает свои программы
		c.SetAL(0x01)
	case 0x03, 0x05, 0x06: // не всплывающая, без горячих клавиш и драйверов устройств
		c.SetAL(0x00)
	case 0x04: // список перехваченных прерываний
		c.SetAL(0x04)
		c.R[cpu.DX] = hle.ROMSeg
		c.R[cpu.BX] = d.amisHooks
	default:
		if fn >= 0x10 && ent.p.Call != nil {
			if ok, err := ent.p.Call(e, fn); ok || err != nil {
				return err
			}
		}
		c.SetAL(0x00) // не реализовано (07h-0Fh зарезервированы)
	}
	return nil
}
