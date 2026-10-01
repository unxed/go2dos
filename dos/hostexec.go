package dos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Команды хоста из встроенной оболочки (docs/HOSTEXEC.md, T18). Это выход из
// песочницы, поэтому включается только Config.HostExec (флаг -host-exec).
//
// Порядок поиска команды в оболочке: внутренние команды, DOS-программы,
// программы хоста (exec.LookPath по первому слову), «Bad command or file name».
// Префикс «!» отправляет строку хосту сразу. Строка целиком (с конвейерами и
// перенаправлением) выполняется оболочкой хоста: sh -c или cmd /c, в каталоге
// хоста, который соответствует текущему каталогу DOS. Вывод (stdout и stderr)
// перехватывается и пишется в хэндл 1 оболочки; stdin — пустой.

// hostExecTimeout ограничивает команду хоста: пока она работает, эмулятор стоит.
const hostExecTimeout = 60 * time.Second

// hostHas сообщает, есть ли на хосте программа с именем первого слова.
func (d *DOS) hostHas(word string) bool {
	if word == "" || strings.ContainsAny(word, `\:`) {
		return false
	}
	_, err := exec.LookPath(d.e.CP.Decode([]byte(word)))
	return err == nil
}

// hostDir — каталог хоста для текущего каталога DOS текущего диска.
func (d *DOS) hostDir() string {
	if h, _, errc := d.fs.resolve(d.fs.cur, d.fs.cwd[d.fs.cur], false); errc == 0 {
		return h
	}
	return d.fs.drives[d.fs.cur]
}

// hostCommand выполняет строку (OEM) на хосте в каталоге dir и возвращает
// вывод в OEM с CR LF и код возврата. err — команду не удалось запустить.
func (d *DOS) hostCommand(line, dir string) (out []byte, code int, err error) {
	cmdline := d.e.CP.Decode([]byte(line))
	ctx, cancel := context.WithTimeout(context.Background(), hostExecTimeout)
	defer cancel()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/c", cmdline)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", cmdline)
	}
	c.Dir = dir
	c.Env = os.Environ()
	d.e.Note("host: %q in %s", cmdline, c.Dir)
	raw, err := c.CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return nil, 0, err
		}
		if code = ee.ExitCode(); code < 0 || code > 255 {
			code = 255
		}
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n", "\r\n")
	out, _ = d.e.CP.Encode(text)
	return out, code, nil
}

// hostRun выполняет строку на хосте в каталоге текущего каталога DOS; вывод
// идёт в хэндл 1 оболочки, код возврата — в ERRORLEVEL.
func (d *DOS) hostRun(st *shellState, line string) {
	out, code, err := d.hostCommand(line, d.hostDir())
	if err != nil {
		d.shPrint("Cannot run the host command: " + err.Error() + crlf)
		st.level = 255
		return
	}
	d.write(1, out)
	st.level = byte(code)
}

// API для DOS-программ: AMIS DOS-HOST/HOSTEXEC (docs/HOSTEXEC-API.md).

// installHostExecAPI регистрирует провайдера; он есть и при выключенном
// HostExec (тогда вызов отвечает «доступ запрещён»), чтобы программа могла
// отличить «режима нет» от «нет провайдера».
func (d *DOS) installHostExecAPI() {
	d.RegisterAMIS(AMISProvider{
		Manufacturer: "DOS-HOST",
		Product:      "HOSTEXEC",
		Description:  "Run a command on the host",
		Version:      0x0100,
		Call:         d.hostExecCall,
	})
}

func (d *DOS) hostExecCall(e *hle.Env, fn byte) (bool, error) {
	if fn != 0x10 {
		return false, nil
	}
	c := e.CPU
	fail := func(code uint16) (bool, error) {
		c.R[cpu.AX] = code
		e.SetCF(true)
		return true, nil
	}
	if !d.hostExec {
		return fail(errAccess)
	}
	pb := mem.Lin(e.Seg(cpu.DS), c.R[cpu.SI])
	cmdPtr, dirPtr := e.Mem.R32(pb), e.Mem.R32(pb+4)
	line := string(e.Mem.ASCIIZ(mem.Lin(uint16(cmdPtr>>16), uint16(cmdPtr)), 1024))
	dir := d.hostDir()
	if dirPtr != 0 {
		p := e.Mem.ASCIIZ(mem.Lin(uint16(dirPtr>>16), uint16(dirPtr)), 260)
		drive, dp, errc := d.fs.canon(p, false)
		if errc != 0 {
			return fail(errPathNotFound)
		}
		h, _, errc := d.fs.resolve(drive, dp, false)
		if errc != 0 {
			return fail(errPathNotFound)
		}
		if st, err := os.Stat(h); err != nil || !st.IsDir() {
			return fail(errPathNotFound)
		}
		dir = h
	}
	out, code, err := d.hostCommand(line, dir)
	if err != nil {
		return fail(errFileNotFound)
	}
	d.write(1, out)
	c.R[cpu.AX] = uint16(code)
	e.SetCF(false)
	return true, nil
}
