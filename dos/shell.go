package dos

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/unxed/go2dos/cpu"
	"github.com/unxed/go2dos/hle"
	"github.com/unxed/go2dos/mem"
)

// Встроенный командный интерпретатор (COMMAND.COM без файла).
//
// Если программа запускает COMMAND.COM (чаще всего по COMSPEC: «COMMAND.COM
// /C команда»), а такого файла в каталоге хоста нет, EXEC создаёт обычный
// процесс, код которого — 8 байт «ловушка HLE; INT 21h; JMP». Ловушка
// (shellStep) разбирает командную строку; внутренние команды выполняются на
// Go, а внешние программы запускаются настоящим EXEC (INT 21h/4B00h) из этого
// процесса, так что цепочка PSP и возврат кода — как у настоящей оболочки.
// Если файл COMMAND.COM лежит в каталоге (например, FreeCOM), запускается он.
//
// Строки здесь — байты в OEM-кодовой странице, а не Unicode: имена и тексты
// передаются DOS без перекодирования, разбор опирается только на ASCII.

const (
	shellParas  = 0x100 // размер блока процесса оболочки (4 КиБ)
	offShellPB  = 0x200 // блок параметров EXEC
	offShellCmd = 0x240 // имя запускаемой программы (ASCIIZ)
	offShellTl  = 0x2C0 // хвост командной строки для программы
	crlf        = "\r\n"
)

// shellImage — образ COM: trap shellTrap; INT 21h; JMP SHORT назад.
func (d *DOS) shellImage() []byte {
	t := hle.Trap(d.shellTrap)
	return append(append([]byte{}, t...), 0xCD, 0x21, 0xEB, 0xF8)
}

// builtinShell возвращает образ встроенной оболочки, если name — это
// COMMAND.COM, которого нет на диске (readImage уже вернул «файл не найден»).
func (d *DOS) builtinShell(name []byte) *image {
	drive, dp, errc := d.fs.canon(name, false)
	if errc != 0 {
		return nil
	}
	if _, base := splitDir(dp); base != "COMMAND.COM" {
		return nil
	}
	return &image{data: d.shellImage(), path: fmt.Sprintf("%c:%s", 'A'+drive, dp)}
}

type batFrame struct {
	lines []string
	pos   int
	args  [10]string
	echo  bool
}

type shellState struct {
	cmd         string // команда из /C или /K, ещё не выполненная
	interactive bool
	bats        []*batFrame
	level       byte // код возврата последней программы
	waiting     bool // EXEC выполняется
	restore     func()
	prompted    bool
	exit        bool
	exitCode    byte
}

type shRedir struct {
	in, out string
	app     bool
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

func eqASCII(a, b string) bool { return lowerASCII(a) == lowerASCII(b) }

// --- состояние ------------------------------------------------------------------

func (d *DOS) newShell(e *hle.Env) *shellState {
	c := e.CPU
	m := e.Mem
	st := &shellState{}
	n := int(m.R8(mem.Lin(d.psp, 0x80)))
	tail := string(m.Bytes(mem.Lin(d.psp, 0x81), n))
	rest := strings.TrimLeft(tail, " \t")
	st.interactive = true
	for strings.HasPrefix(rest, "/") && len(rest) >= 2 {
		sw := upperASCII(rest[1:2])
		if sw == "C" || sw == "K" {
			st.cmd = strings.TrimLeft(rest[2:], " \t")
			st.interactive = sw == "K"
			rest = ""
			break
		}
		// Другие ключи (/P, /E:n, /MSG, ...) ничего не меняют.
		if i := strings.IndexAny(rest, " \t"); i >= 0 {
			rest = strings.TrimLeft(rest[i:], " \t")
		} else {
			rest = ""
		}
	}
	e.Note("built-in COMMAND.COM: tail %q", tail)
	// Блок процесса сжимается, чтобы дочерним программам хватило памяти.
	d.resize(d.psp, shellParas)
	c.R[cpu.SP] = shellParas*16 - 2
	c.Write16(cpu.SS, c.R[cpu.SP], 0)
	if st.interactive && st.cmd == "" {
		d.shPrint("go2dos built-in command interpreter" + crlf + crlf)
	}
	return st
}

// shPrint пишет в стандартный вывод (handle 1) оболочки, то есть с
// учётом перенаправления.
func (d *DOS) shPrint(s string) { d.write(1, []byte(s)) }

func (d *DOS) shellStep(e *hle.Env) error {
	c := e.CPU
	st := d.shells[d.psp]
	if st == nil {
		st = d.newShell(e)
		d.shells[d.psp] = st
	}
	if st.waiting {
		st.waiting = false
		if st.restore != nil {
			st.restore()
			st.restore = nil
		}
		if c.Flags&cpu.FlagCF != 0 {
			d.shPrint("Cannot run the program: " + errText(c.R[cpu.AX]) + crlf)
			st.level = 255
		} else {
			st.level = d.exit
		}
	}
	for !st.exit {
		line, ok, wait := d.shellNextLine(st)
		if wait {
			e.Idle()
			return cpu.ErrRetry
		}
		if !ok {
			st.exit = true
			st.exitCode = st.level
			break
		}
		if d.shellRun(e, st, line) {
			return nil // EXEC подготовлен: следующая инструкция — INT 21h
		}
	}
	delete(d.shells, d.psp)
	d.terminate(st.exitCode, false)
	return nil
}

func (d *DOS) shellNextLine(st *shellState) (line string, ok, wait bool) {
	for len(st.bats) > 0 {
		b := st.bats[len(st.bats)-1]
		if b.pos < len(b.lines) {
			l := b.lines[b.pos]
			b.pos++
			l = d.batExpand(b, l)
			if b.echo && !strings.HasPrefix(strings.TrimSpace(l), "@") && strings.TrimSpace(l) != "" {
				d.shPrint(d.shPrompt() + l + crlf)
			}
			return l, true, false
		}
		st.bats = st.bats[:len(st.bats)-1]
	}
	if st.cmd != "" {
		l := st.cmd
		st.cmd = ""
		return l, true, false
	}
	if !st.interactive {
		return "", false, false
	}
	if !st.prompted {
		d.shPrint(d.shPrompt())
		st.prompted = true
	}
	if !d.lineInput(126, false) {
		return "", false, true
	}
	l := string(d.line)
	d.line = d.line[:0]
	d.lineDone = false
	d.conWrite([]byte{0x0A})
	st.prompted = false
	return l, true, false
}

// --- окружение ---------------------------------------------------------------------

// envParse читает окружение процесса: строки NAME=VALUE и «хвост» (слово и
// имя программы) после завершающего двойного нуля.
func (d *DOS) envParse() (vars []string, tail []byte) {
	m := d.e.Mem
	seg := m.R16(mem.Lin(d.psp, 0x2C))
	if seg == 0 {
		return nil, nil
	}
	a := mem.Lin(seg, 0)
	var cur []byte
	for i := uint32(0); i < 0x8000; i++ {
		b := m.R8(a + i)
		if b == 0 {
			if len(cur) == 0 {
				tail = append(tail, m.Bytes(a+i+1, 130)...)
				return vars, tail
			}
			vars = append(vars, string(cur))
			cur = nil
			continue
		}
		cur = append(cur, b)
	}
	return vars, nil
}

// envStore записывает окружение в новый блок и освобождает старый.
func (d *DOS) envStore(vars []string, tail []byte) bool {
	var buf []byte
	for _, v := range vars {
		buf = append(buf, v...)
		buf = append(buf, 0)
	}
	buf = append(buf, 0)
	buf = append(buf, tail...)
	seg, _, errc := d.alloc(uint16((len(buf)+15)/16), d.psp)
	if errc != 0 {
		return false
	}
	m := d.e.Mem
	m.SetBytes(mem.Lin(seg, 0), buf)
	old := m.R16(mem.Lin(d.psp, 0x2C))
	m.W16(mem.Lin(d.psp, 0x2C), seg)
	if old != 0 {
		d.free(old)
	}
	return true
}

func (d *DOS) envGet(name string) (string, bool) {
	vars, _ := d.envParse()
	for _, v := range vars {
		if i := strings.IndexByte(v, '='); i >= 0 && eqASCII(v[:i], name) {
			return v[i+1:], true
		}
	}
	return "", false
}

func (d *DOS) envSet(name, value string, del bool) bool {
	vars, tail := d.envParse()
	name = upperASCII(name)
	var out []string
	for _, v := range vars {
		if i := strings.IndexByte(v, '='); i >= 0 && upperASCII(v[:i]) == name {
			continue
		}
		out = append(out, v)
	}
	if !del {
		out = append(out, name+"="+value)
	}
	return d.envStore(out, tail)
}

func (d *DOS) shPrompt() string {
	p, ok := d.envGet("PROMPT")
	if !ok {
		p = "$N$G"
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] != '$' || i+1 >= len(p) {
			b.WriteByte(p[i])
			continue
		}
		i++
		switch upperASCII(p[i : i+1]) {
		case "P":
			b.WriteString(fmt.Sprintf("%c:%s", 'A'+d.fs.cur, d.fs.cwd[d.fs.cur]))
		case "N":
			b.WriteString(fmt.Sprintf("%c:", 'A'+d.fs.cur))
		case "G":
			b.WriteByte('>')
		case "L":
			b.WriteByte('<')
		case "B":
			b.WriteByte('|')
		case "Q":
			b.WriteByte('=')
		case "_":
			b.WriteString(crlf)
		case "$":
			b.WriteByte('$')
		}
	}
	return b.String()
}

// --- пакетные файлы ------------------------------------------------------------------

// batExpand подставляет %0-%9, %% и %NAME%.
func (d *DOS) batExpand(b *batFrame, l string) string {
	if !strings.Contains(l, "%") {
		return l
	}
	var out strings.Builder
	for i := 0; i < len(l); i++ {
		if l[i] != '%' || i+1 >= len(l) {
			out.WriteByte(l[i])
			continue
		}
		n := l[i+1]
		switch {
		case n == '%':
			out.WriteByte('%')
			i++
		case n >= '0' && n <= '9':
			out.WriteString(b.args[n-'0'])
			i++
		default:
			j := strings.IndexByte(l[i+1:], '%')
			if j < 0 {
				out.WriteByte('%')
				continue
			}
			v, _ := d.envGet(l[i+1 : i+1+j])
			out.WriteString(v)
			i += j + 1
		}
	}
	return out.String()
}

// --- разбор строки ------------------------------------------------------------------

// splitRedir выделяет <, >, >> вне кавычек. Конвейер (|) не поддерживается.
func splitRedir(line string) (string, shRedir, bool) {
	var r shRedir
	var out strings.Builder
	quote := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '"':
			quote = !quote
			out.WriteByte(ch)
		case quote:
			out.WriteByte(ch)
		case ch == '|':
			return "", r, false
		case ch == '<' || ch == '>':
			app := false
			if ch == '>' && i+1 < len(line) && line[i+1] == '>' {
				app = true
				i++
			}
			j := i + 1
			for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			k := j
			for k < len(line) && line[k] != ' ' && line[k] != '\t' && line[k] != '<' && line[k] != '>' {
				k++
			}
			if ch == '<' {
				r.in = line[j:k]
			} else {
				r.out, r.app = line[j:k], app
			}
			i = k - 1
		default:
			out.WriteByte(ch)
		}
	}
	return strings.TrimSpace(out.String()), r, true
}

var shellInternal = map[string]bool{
	"DIR": true, "CD": true, "CHDIR": true, "MD": true, "MKDIR": true, "RD": true, "RMDIR": true,
	"SET": true, "ECHO": true, "TYPE": true, "VER": true, "EXIT": true, "REM": true, "PATH": true,
	"PROMPT": true, "DEL": true, "ERASE": true, "REN": true, "RENAME": true, "COPY": true,
	"CALL": true, "GOTO": true, "IF": true, "FOR": true, "PAUSE": true, "CLS": true, "SHIFT": true,
}

// redirect выполняет перенаправление вывода/ввода оболочки; возвращает
// функцию, которая всё возвращает как было.
func (d *DOS) redirect(r shRedir) (restore func(), errc uint16) {
	var undo []func()
	restore = func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	m := d.e.Mem
	set := func(slot uint32, path string, mode byte, create, trunc, app bool) uint16 {
		h, errc := d.open([]byte(path), mode, create, trunc, false)
		if errc != 0 {
			return errc
		}
		of, _ := d.handle(h)
		if app && of.f != nil {
			of.f.Seek(0, io.SeekEnd)
		}
		a, _ := d.jft()
		idx := m.R8(a + uint32(h))
		old := m.R8(a + slot)
		m.W8(a+slot, idx)
		of.refs++
		d.closeHandle(h)
		undo = append(undo, func() {
			d.closeHandle(uint16(slot))
			a, _ := d.jft()
			m.W8(a+slot, old)
		})
		return 0
	}
	if r.in != "" {
		if errc = set(0, r.in, 0, false, false, false); errc != 0 {
			restore()
			return func() {}, errc
		}
	}
	if r.out != "" {
		if errc = set(1, r.out, 1, true, !r.app, r.app); errc != 0 {
			restore()
			return func() {}, errc
		}
	}
	return restore, 0
}

// shellRun выполняет одну строку. Возвращает true, если подготовлен EXEC.
func (d *DOS) shellRun(e *hle.Env, st *shellState, line string) bool {
	line = strings.TrimSpace(line)
	for strings.HasPrefix(line, "@") {
		line = strings.TrimSpace(line[1:])
	}
	if line == "" || line[0] == ':' {
		return false
	}
	if strings.HasPrefix(line, "!") { // the host's command
		if !d.hostExec {
			d.shPrint("Host commands are disabled (go2dos -host-exec)" + crlf)
			st.level = 255
		} else {
			d.hostRun(st, strings.TrimSpace(line[1:]))
		}
		return false
	}
	orig := line
	line, redir, ok := splitRedir(line)
	if !ok { // a pipe: only the host shell has them
		if w := firstWord(orig); d.hostExec && !shellInternal[upperASCII(w)] && d.hostHas(w) {
			if _, kind := d.findProgram(w); kind == "" {
				d.hostRun(st, orig)
				return false
			}
		}
		d.shPrint("Pipes are not supported by the built-in COMMAND.COM" + crlf)
		st.level = 255
		return false
	}
	if line == "" {
		return false
	}
	// Имя команды: до пробела или '/'.
	end := strings.IndexAny(line, " \t/")
	if end < 0 {
		end = len(line)
	}
	tok := line[:end]
	rest := line[end:]
	if len(tok) == 2 && tok[1] == ':' && ((tok[0] >= 'A' && tok[0] <= 'Z') || (tok[0] >= 'a' && tok[0] <= 'z')) {
		l := upperASCII(tok[:1])[0] - 'A'
		if d.fs.drives[l] == "" {
			d.shPrint("Invalid drive specification" + crlf)
			st.level = 1
		} else {
			d.fs.cur = int(l)
		}
		return false
	}
	// Внутренняя команда: имя из букв/цифр, дальше конец, '.' или '\'.
	n := 0
	for n < len(tok) && (tok[n] >= 'A' && tok[n] <= 'Z' || tok[n] >= 'a' && tok[n] <= 'z' || tok[n] >= '0' && tok[n] <= '9' || tok[n] == '_') {
		n++
	}
	name := upperASCII(tok[:n])
	if shellInternal[name] && (n == len(tok) || tok[n] == '.' || tok[n] == '\\') {
		args := tok[n:] + rest
		restore, errc := d.redirect(redir)
		if errc != 0 {
			d.shPrint("Cannot redirect: " + errText(errc) + crlf)
			st.level = 1
			return false
		}
		d.shellInternalCmd(e, st, name, args)
		restore()
		return false
	}
	// Внешняя программа.
	path, kind := d.findProgram(tok)
	if kind == "" {
		if d.hostExec && d.hostHas(tok) {
			d.hostRun(st, orig)
			return false
		}
		d.shPrint("Bad command or file name" + crlf)
		st.level = 255
		return false
	}
	tail := strings.TrimRight(rest, " \t")
	if kind == "BAT" {
		d.runBat(st, path, tok, tail)
		return false
	}
	restore, errc := d.redirect(redir)
	if errc != 0 {
		d.shPrint("Cannot redirect: " + errText(errc) + crlf)
		st.level = 1
		return false
	}
	st.restore = restore
	st.waiting = true
	if len(tail) > 126 {
		tail = tail[:126]
	}
	m := e.Mem
	psp := d.psp
	m.SetBytes(mem.Lin(psp, offShellCmd), append([]byte(path), 0))
	m.W8(mem.Lin(psp, offShellTl), byte(len(tail)))
	m.SetBytes(mem.Lin(psp, offShellTl+1), append([]byte(tail), 0x0D))
	pb := mem.Lin(psp, offShellPB)
	m.W16(pb, 0)
	m.W16(pb+2, offShellTl)
	m.W16(pb+4, psp)
	m.W16(pb+6, 0x5C)
	m.W16(pb+8, psp)
	m.W16(pb+10, 0x6C)
	m.W16(pb+12, psp)
	c := e.CPU
	c.R[cpu.AX] = 0x4B00
	c.R[cpu.DX] = offShellCmd
	c.R[cpu.BX] = offShellPB
	c.SetSeg(cpu.DS, psp)
	c.SetSeg(cpu.ES, psp)
	e.Note("run %s%s", path, tail)
	return true
}

// findProgram ищет COM, EXE или BAT: с расширением — как есть, без него —
// по порядку COM, EXE, BAT; без пути — в текущем каталоге и по PATH.
func (d *DOS) findProgram(tok string) (path, kind string) {
	var dirs []string
	if strings.ContainsAny(tok, `\:`) {
		dirs = []string{""}
	} else {
		dirs = []string{""}
		if p, ok := d.envGet("PATH"); ok {
			for _, x := range strings.Split(p, ";") {
				if x = strings.TrimSpace(x); x != "" {
					dirs = append(dirs, strings.TrimRight(x, `\`)+`\`)
				}
			}
		}
	}
	_, last := splitDirLoose(tok)
	exts := []string{".COM", ".EXE", ".BAT"}
	if i := strings.LastIndexByte(last, '.'); i >= 0 {
		exts = []string{""}
		switch upperASCII(last[i:]) {
		case ".COM", ".EXE", ".BAT":
		default:
			return "", ""
		}
	}
	for _, dir := range dirs {
		for _, x := range exts {
			cand := dir + tok + x
			drive, dp, errc := d.fs.canon([]byte(cand), false)
			if errc != 0 {
				continue
			}
			host, _, errc := d.fs.resolve(drive, dp, false)
			if errc != 0 {
				continue
			}
			if st, err := os.Stat(host); err != nil || st.IsDir() {
				continue
			}
			_, base := splitDir(dp)
			k := upperASCII(base[strings.LastIndexByte(base, '.')+1:])
			return fmt.Sprintf("%c:%s", 'A'+drive, dp), k
		}
	}
	return "", ""
}

// firstWord — первое слово строки (до пробела, '/', '|', '<', '>').
func firstWord(line string) string {
	if i := strings.IndexAny(line, " \t/|<>"); i >= 0 {
		return line[:i]
	}
	return line
}

func splitDirLoose(p string) (string, string) {
	i := strings.LastIndexAny(p, `\:`)
	return p[:i+1], p[i+1:]
}

func (d *DOS) runBat(st *shellState, path, name, tail string) {
	drive, dp, _ := d.fs.canon([]byte(path), false)
	host, _, errc := d.fs.resolve(drive, dp, false)
	if errc != 0 {
		d.shPrint("Batch file not found" + crlf)
		return
	}
	data, err := d.fs.readFile(host)
	if err != nil {
		d.shPrint("Batch file not found" + crlf)
		return
	}
	text := strings.TrimRight(string(data), "\x1a")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	b := &batFrame{lines: strings.Split(text, "\n"), echo: true}
	b.args[0] = name
	for i, a := range strings.Fields(tail) {
		if i+1 > 9 {
			break
		}
		b.args[i+1] = a
	}
	st.bats = append(st.bats, b)
}

// --- внутренние команды ---------------------------------------------------------------

func (d *DOS) shellInternalCmd(e *hle.Env, st *shellState, name, args string) {
	a := strings.TrimSpace(args)
	errOut := func(errc uint16, what string) {
		d.shPrint(what + ": " + errText(errc) + crlf)
		st.level = 1
	}
	switch name {
	case "REM":
	case "ECHO":
		switch {
		case a == "" && args == "":
			d.shPrint("ECHO is on" + crlf)
		case eqASCII(a, "ON") || eqASCII(a, "OFF"):
			if len(st.bats) > 0 {
				st.bats[len(st.bats)-1].echo = eqASCII(a, "ON")
			}
		default:
			s := args
			if strings.HasPrefix(s, " ") || strings.HasPrefix(s, ".") {
				s = s[1:]
			}
			d.shPrint(s + crlf)
		}
	case "VER":
		d.shPrint(crlf + "go2dos built-in COMMAND.COM (DOS " + strconv.Itoa(VersionMajor) + "." + strconv.Itoa(VersionMinor) + ")" + crlf)
	case "EXIT":
		st.exit = true
		if n, err := strconv.Atoi(a); err == nil {
			st.exitCode = byte(n)
		} else {
			st.exitCode = st.level
		}
	case "CD", "CHDIR":
		if a == "" {
			d.shPrint(fmt.Sprintf("%c:%s%s", 'A'+d.fs.cur, d.fs.cwd[d.fs.cur], crlf))
		} else if errc := d.chdir(d.shPath(shOne(a))); errc != 0 {
			d.shPrint("Invalid directory" + crlf)
			st.level = 1
		}
	case "MD", "MKDIR":
		if errc := d.shMkdir(shOne(a)); errc != 0 {
			errOut(errc, "Unable to create directory")
		}
	case "RD", "RMDIR":
		if errc := d.rmdir(d.shPath(shOne(a))); errc != 0 {
			errOut(errc, "Invalid path, not directory, or directory not empty")
		}
	case "DEL", "ERASE":
		if strings.ContainsAny(a, "*?") {
			d.shPrint("Wildcards are not supported by DEL in the built-in COMMAND.COM" + crlf)
			st.level = 1
		} else if errc := d.unlink(d.shPath(shOne(a))); errc != 0 {
			errOut(errc, "Cannot delete")
		}
	case "REN", "RENAME":
		f := shArgs(a)
		if len(f) != 2 {
			d.shPrint("Required parameter missing" + crlf)
			st.level = 1
		} else if errc := d.shRename(f[0], f[1]); errc != 0 {
			errOut(errc, "Cannot rename")
		}
	case "SET":
		d.shSet(st, a)
	case "PATH":
		if a == "" {
			v, ok := d.envGet("PATH")
			if ok {
				d.shPrint("PATH=" + v + crlf)
			} else {
				d.shPrint("No Path" + crlf)
			}
		} else {
			d.envSet("PATH", strings.TrimPrefix(a, "="), false)
		}
	case "PROMPT":
		if a == "" {
			d.envSet("PROMPT", "", true)
		} else {
			d.envSet("PROMPT", a, false)
		}
	case "TYPE":
		d.shCopy(st, shOne(a), "")
	case "COPY":
		f := shArgs(a)
		if len(f) != 2 || strings.Contains(a, "+") {
			d.shPrint("Only COPY source destination is supported by the built-in COMMAND.COM" + crlf)
			st.level = 1
			return
		}
		d.shCopy(st, f[0], f[1])
	case "DIR":
		d.shDir(st, a)
	case "CALL":
		d.shPrint("CALL is not supported by the built-in COMMAND.COM" + crlf)
		st.level = 1
	default: // GOTO, IF, FOR, PAUSE, CLS, SHIFT
		d.shPrint(name + " is not supported by the built-in COMMAND.COM" + crlf)
		st.level = 1
	}
}

func (d *DOS) shSet(st *shellState, a string) {
	if a == "" {
		vars, _ := d.envParse()
		for _, v := range vars {
			d.shPrint(v + crlf)
		}
		return
	}
	i := strings.IndexByte(a, '=')
	if i <= 0 {
		v, ok := d.envGet(a)
		if !ok {
			d.shPrint("Environment variable " + a + " not defined" + crlf)
			st.level = 1
			return
		}
		d.shPrint(upperASCII(a) + "=" + v + crlf)
		return
	}
	if !d.envSet(strings.TrimSpace(a[:i]), a[i+1:], a[i+1:] == "") {
		d.shPrint("Out of environment space" + crlf)
		st.level = 1
	}
}

// shCopy копирует файл src в dst; если dst пуст — в стандартный вывод (TYPE).
func (d *DOS) shCopy(st *shellState, src, dst string) {
	if src == "" {
		d.shPrint("Required parameter missing" + crlf)
		st.level = 1
		return
	}
	in, errc := d.shOpen(src, false)
	if errc != 0 {
		d.shPrint("File not found - " + src + crlf)
		st.level = 1
		return
	}
	defer d.closeHandle(in)
	out := uint16(1)
	if dst != "" {
		h, errc := d.shOpen(dst, true)
		if errc != 0 {
			d.shPrint("Cannot create " + dst + ": " + errText(errc) + crlf)
			st.level = 1
			return
		}
		defer d.closeHandle(h)
		out = h
	}
	buf := make([]byte, 4096)
	for {
		n, errc := d.read(in, buf)
		if errc != 0 || n == 0 {
			break
		}
		d.write(out, buf[:n])
	}
	if dst != "" {
		d.shPrint("        1 file(s) copied" + crlf)
	}
}

func commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func (d *DOS) shDir(st *shellState, a string) {
	var arg string
	for _, f := range shArgs(a) {
		if !strings.HasPrefix(f, "/") {
			arg = f
		}
	}
	if arg == "" {
		arg = "*.*"
	}
	drive, dp, errc := d.fs.canon(d.shPath(arg), true)
	if errc != 0 {
		d.shPrint("Invalid drive specification" + crlf)
		st.level = 1
		return
	}
	dir, mask := dp, "*.*"
	isDir := false
	if !strings.ContainsAny(dp, "*?") {
		if h, _, ec := d.fs.resolve(drive, dp, false); ec == 0 {
			if fi, err := os.Stat(h); err == nil && fi.IsDir() {
				isDir = true
			}
		}
	}
	if !isDir {
		dir, mask = splitDir(dp)
	}
	host, _, errc := d.fs.resolve(drive, dir, false)
	if errc != 0 {
		d.shPrint("File not found" + crlf)
		st.level = 1
		return
	}
	ix, errc := d.fs.index(host)
	if errc != 0 {
		d.shPrint("File not found" + crlf)
		st.level = 1
		return
	}
	label := d.fs.labels[drive]
	if label == "" {
		d.shPrint(fmt.Sprintf(" Volume in drive %c has no label%s", 'A'+drive, crlf))
	} else {
		d.shPrint(fmt.Sprintf(" Volume in drive %c is %s%s", 'A'+drive, label, crlf))
	}
	ser := d.fs.serial(drive)
	d.shPrint(fmt.Sprintf(" Volume Serial Number is %04X-%04X%s%s", ser>>16, ser&0xFFFF, crlf, crlf))
	d.shPrint(fmt.Sprintf(" Directory of %c:%s%s%s", 'A'+drive, dir, crlf, crlf))
	pat := fcbName(mask)
	var files, dirs int
	var bytes int64
	var names []dirEntry
	for _, en := range ix.entries {
		if attrOf(en)&attrHidden != 0 || !(fcbMatch(pat, fcbName(en.dos)) || wildMatch(mask, en.host)) {
			continue
		}
		names = append(names, en)
	}
	sort.SliceStable(names, func(i, j int) bool { return names[i].dos < names[j].dos })
	line := func(base, ext, size string, en dirEntry) {
		t := en.info.ModTime()
		h, ap := t.Hour(), 'a'
		if h >= 12 {
			ap = 'p'
		}
		if h > 12 {
			h -= 12
		}
		if h == 0 {
			h = 12
		}
		long := ""
		if l := d.shLongName(en); l != "" {
			long = " " + l
		}
		d.shPrint(fmt.Sprintf("%-8s %-3s %10s  %02d-%02d-%02d %2d:%02d%c%s%s", base, ext, size,
			int(t.Month()), t.Day(), t.Year()%100, h, t.Minute(), ap, long, crlf))
	}
	if dir != `\` {
		if mask == "*.*" {
			d.shPrint(fmt.Sprintf("%-8s %-3s %10s%s", ".", "", "<DIR>", crlf))
			d.shPrint(fmt.Sprintf("%-8s %-3s %10s%s", "..", "", "<DIR>", crlf))
			dirs += 2
		}
	}
	for _, en := range names {
		base, ext := en.dos, ""
		if i := strings.LastIndexByte(en.dos, '.'); i >= 0 {
			base, ext = en.dos[:i], en.dos[i+1:]
		}
		if en.info.IsDir() {
			line(base, ext, "<DIR>", en)
			dirs++
		} else {
			line(base, ext, commas(en.info.Size()), en)
			files++
			bytes += en.info.Size()
		}
	}
	if files+dirs == 0 {
		d.shPrint("File not found" + crlf)
		st.level = 1
		return
	}
	d.shPrint(fmt.Sprintf("%9d file(s) %14s bytes%s", files, commas(bytes), crlf))
	d.shPrint(fmt.Sprintf("%9d dir(s)  %14s bytes free%s", dirs, "1,073,741,824", crlf))
}
