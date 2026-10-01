package dos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
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

// hostRun выполняет строку на хосте; код возврата идёт в ERRORLEVEL оболочки.
func (d *DOS) hostRun(st *shellState, line string) {
	cmdline := d.e.CP.Decode([]byte(line))
	ctx, cancel := context.WithTimeout(context.Background(), hostExecTimeout)
	defer cancel()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/c", cmdline)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", cmdline)
	}
	c.Dir = d.hostDir()
	c.Env = os.Environ()
	d.e.Note("host: %q in %s", cmdline, c.Dir)
	out, err := c.CombinedOutput()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			d.shPrint("Cannot run the host command: " + err.Error() + crlf)
			st.level = 255
			return
		}
		if code = ee.ExitCode(); code < 0 || code > 255 {
			code = 255
		}
	}
	text := strings.ReplaceAll(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n", "\r\n")
	b, _ := d.e.CP.Encode(text)
	d.write(1, b)
	st.level = byte(code)
}
