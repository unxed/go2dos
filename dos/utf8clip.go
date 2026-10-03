package dos

import (
	"strings"
	"unicode/utf8"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
)

// Провайдер UTF-8 текста буфера обмена (docs/UTF8CLIPBOARD.md, версия 1.0).
// Находится через AMIS (amis.go): производитель «DOS-UTF8», продукт «CLIPBRD».
// Устроен как провайдер имён (utf8names.go): выбор кодировки делает процесс.
//
//	AL=10h BX=65001|0: UTF-8 (65001) или OEM (0, по умолчанию) для текста
//	                   буфера обмена вызывающего процесса (выход: AL=FFh, BX —
//	                   прежнее значение; AL=00h — значение не поддерживается);
//	AL=11h:            AL=FFh, BX — текущее значение.
//
// Процесс — тот, чей PSP текущий. Режим кончается вместе с процессом и не
// наследуется через EXEC. В режиме UTF-8 форматы WinOldAp CF_TEXT (01h) и
// CF_OEMTEXT (07h) несут UTF-8 вместо OEM: размер 1704h — в байтах UTF-8,
// данные 1705h — UTF-8 с CR LF и завершающим нулём, 1703h принимает UTF-8
// (недопустимые последовательности заменяются на U+FFFD). Провайдер есть
// только там, где есть сам сервер буфера обмена (Config.Clipboard != nil).

func (d *DOS) installUTF8Clip() {
	d.clipUTF8 = map[uint16]bool{}
	if d.clip == nil {
		return
	}
	d.RegisterAMIS(AMISProvider{
		Manufacturer: "DOS-UTF8",
		Product:      "CLIPBRD",
		Description:  "UTF-8 text of the clipboard for DOS programs",
		Version:      0x0100,
		Call:         d.utf8ClipCall,
	})
}

func (d *DOS) utf8ClipCall(e *hle.Env, fn byte) (bool, error) {
	c := e.CPU
	cur := uint16(0)
	if d.clipUTF8[d.psp] {
		cur = cpUTF8
	}
	switch fn {
	case 0x10:
		switch c.R[cpu.BX] {
		case cpUTF8:
			d.clipUTF8[d.psp] = true
		case 0:
			delete(d.clipUTF8, d.psp)
		default:
			c.SetAL(0x00) // не поддерживается, режим не менялся
			return true, nil
		}
		c.SetAL(0xFF)
		c.R[cpu.BX] = cur
	case 0x11:
		c.SetAL(0xFF)
		c.R[cpu.BX] = cur
	default:
		return false, nil
	}
	return true, nil
}

// clipEncode — текст буфера для процесса: UTF-8 или OEM (LF → CR LF уже сделан).
func (d *DOS) clipEncode(s string) []byte {
	if d.clipUTF8[d.psp] {
		return []byte(s)
	}
	b, _ := d.e.CP.Encode(s)
	return b
}

// clipDecode — байты, записанные процессом (до нуля), в Unicode-текст хоста.
func (d *DOS) clipDecode(b []byte) string {
	if d.clipUTF8[d.psp] {
		return strings.ToValidUTF8(string(b), "�")
	}
	return d.e.CP.Decode(b)
}

// clipCut обрезает текст до clipMax-1 байт; в UTF-8 не посреди символа.
func (d *DOS) clipCut(b []byte) []byte {
	if len(b) < clipMax {
		return b
	}
	b = b[:clipMax-1]
	if d.clipUTF8[d.psp] {
		for len(b) > 0 && !utf8.Valid(b) {
			b = b[:len(b)-1]
		}
	}
	return b
}
