package dos

import (
	"strings"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Буфер обмена: сервер WinOldAp, INT 2Fh AX=17xxh (RBIL, «MS Windows
// WINOLDAP»). Клиенты: GNU Emacs для MS-DOS (w16select.c), 4DOS, WCLIP.
//
// Порядок работы клиента (w16select.c): 1700h (версия) → 1701h открыть →
// 1702h очистить → 1703h записать → 1708h закрыть; чтение: 1704h размер →
// 1705h данные. Текст — байты с CR LF и завершающим нулём; клиент при
// чтении останавливается на первом нуле, потому что Windows округляет размер
// вверх до 32 и не обновляет его после удаления хвостовых пробелов.

// Clipboard — буфер обмена хоста (Unicode-текст). Без него (nil) INT 2Fh/17xxh
// отвечает «не установлено», как и без Windows.
type Clipboard interface {
	GetText() (string, error)
	SetText(string) error
}

// MemClipboard — буфер в памяти: для тестов и для хостов без своего буфера.
type MemClipboard struct{ Text string }

func (c *MemClipboard) GetText() (string, error) { return c.Text, nil }
func (c *MemClipboard) SetText(s string) error   { c.Text = s; return nil }

// Форматы WinOldAp (RBIL, табл. 02723), которые мы поддерживаем.
const (
	cfText    = 0x01
	cfOEMText = 0x07
)

const (
	clipMax     = 1 << 20 // больше клиенту не обещаем (1709h)
	winOldApVer = 0x0A03  // AL=3, AH=10: «Windows 3.10» (предположение)
)

// clipFormatOK: оба текстовых формата трактуются как OEM (DESIGN §9).
func clipFormatOK(f uint16) bool { return f == cfText || f == cfOEMText }

// clipBytes — текст буфера как его отдаёт сервер: OEM-байты, LF → CR LF.
func (d *DOS) clipBytes() ([]byte, bool) {
	s, err := d.clip.GetText()
	if err != nil || s == "" {
		return nil, false
	}
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
	b, _ := d.e.CP.Encode(s)
	if len(b) >= clipMax {
		b = b[:clipMax-1]
	}
	return b, true
}

// winOldAp обслуживает INT 2Fh AH=17h; false — вызов не наш.
func (d *DOS) winOldAp(e *hle.Env) (bool, error) {
	if d.clip == nil || e.CPU.AH() != 0x17 {
		return false, nil
	}
	c := e.CPU
	ret := func(v uint16) { c.R[cpu.AX] = v }
	flag := func(ok bool) {
		if ok {
			ret(1)
		} else {
			ret(0)
		}
	}
	switch c.AL() {
	case 0x00:
		ret(winOldApVer)
	case 0x01: // открыть: 0 — уже открыт
		flag(!d.clipOpen)
		d.clipOpen = true
	case 0x02: // очистить (как в Windows, только у открытого буфера)
		if !d.clipOpen {
			ret(0)
			return true, nil
		}
		flag(d.clip.SetText("") == nil)
	case 0x03: // записать: DX формат, ES:BX данные, SI:CX размер
		size := int(c.R[cpu.SI])<<16 | int(c.R[cpu.CX])
		if !d.clipOpen || !clipFormatOK(c.R[cpu.DX]) || size > clipMax {
			ret(0)
			return true, nil
		}
		b := e.Mem.Bytes(mem.Lin(e.Seg(cpu.ES), c.R[cpu.BX]), size)
		if i := strings.IndexByte(string(b), 0); i >= 0 {
			b = b[:i]
		}
		s := strings.ReplaceAll(d.e.CP.Decode(b), "\r\n", "\n")
		flag(d.clip.SetText(s) == nil)
	case 0x04: // размер данных формата DX: с завершающим нулём; 0 — данных нет
		var n int
		if clipFormatOK(c.R[cpu.DX]) {
			if b, ok := d.clipBytes(); ok {
				n = len(b) + 1
			}
		}
		c.R[cpu.AX], c.R[cpu.DX] = uint16(n), uint16(n>>16)
	case 0x05: // данные формата DX в ES:BX
		b, ok := d.clipBytes()
		if !clipFormatOK(c.R[cpu.DX]) || !ok {
			ret(0)
			return true, nil
		}
		e.Mem.SetBytes(mem.Lin(e.Seg(cpu.ES), c.R[cpu.BX]), append(b, 0))
		ret(1)
	case 0x08: // закрыть
		flag(d.clipOpen)
		d.clipOpen = false
	case 0x09: // уплотнить: места всегда хватает до clipMax
		n := uint32(clipMax)
		c.R[cpu.AX], c.R[cpu.DX] = uint16(n), uint16(n>>16)
	default:
		return true, hle.Unsupported("INT 2Fh AX=%04Xh (WinOldAp)", c.R[cpu.AX])
	}
	return true, nil
}
