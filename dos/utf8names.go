package dos

import (
	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Провайдер UTF-8 имён файлов (docs/UTF8NAMES.md, версия 1.0; M10, T09).
// Находится через AMIS (amis.go): производитель «DOS-UTF8», продукт «NAMES».
//
//	AL=10h BX=65001|0: включить/выключить UTF-8 для вызывающего процесса
//	                   (выход: AL=FFh, BX — прежнее значение; AL=00h — значение
//	                   не поддерживается, ничего не изменилось);
//	AL=11h:            AL=FFh, BX — текущее значение.
//
// Процесс — тот, чей PSP текущий. Режим кончается вместе с процессом и не
// наследуется через EXEC (его хранит карта по PSP, дочерний процесс — другой
// PSP). В режиме: длинные имена и пути (71xx) — UTF-8, недопустимый UTF-8 на
// входе — ошибка 3; короткие имена — только ASCII (имя, которому нужны другие
// символы, получает алиас NAME~N.EXT), а значит и классические вызовы видят
// только ASCII; имя длиннее 255 байт в UTF-8 выдаётся коротким; таблицы
// заглавных букв (6502h, 6504h) отображают 80h-FFh сами в себя, а 6520h-6522h
// эти байты не меняют. Остальное остаётся в OEM-кодовой странице.

const cpUTF8 = 65001

// Таблицы заглавных букв «сами в себя» для процессов в режиме UTF-8.
const (
	offUpperID  = 0x0C00
	offFUpperID = 0x0C90
)

func (d *DOS) installUTF8Names() {
	d.utf8 = map[uint16]bool{}
	d.fs.utf8Mode = func() bool { return d.utf8[d.psp] }
	m := d.e.Mem
	for _, off := range []uint16{offUpperID, offFUpperID} {
		a := mem.Lin(dataSeg, off)
		m.W16(a, 128)
		for i := 0; i < 128; i++ {
			m.W8(a+2+uint32(i), byte(0x80+i))
		}
	}
	d.RegisterAMIS(AMISProvider{
		Manufacturer: "DOS-UTF8",
		Product:      "NAMES",
		Description:  "UTF-8 file names for DOS programs",
		Version:      0x0100,
		Call:         d.utf8Call,
	})
}

func (d *DOS) utf8Call(e *hle.Env, fn byte) (bool, error) {
	c := e.CPU
	cur := uint16(0)
	if d.utf8[d.psp] {
		cur = cpUTF8
	}
	switch fn {
	case 0x10:
		switch c.R[cpu.BX] {
		case cpUTF8:
			d.utf8[d.psp] = true
		case 0:
			delete(d.utf8, d.psp)
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
