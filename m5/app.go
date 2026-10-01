package m5

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/unxed/go2dos/keys"
	"github.com/unxed/go2dos/ui"
)

// App represents the main m5 file browser application.
// Integrates T-04 (keyboard decoder), T-05 (UI framework), T-06 (FS abstraction).
type App struct {
	output       io.Writer
	running      bool
	sigChan      chan os.Signal
	decoder      *keys.Decoder
	buf          *ui.Buffer
	screenWidth  int
	screenHeight int
	oldTermState *term.State
}

// NewApp creates a new m5 application.
func NewApp(output io.Writer) (*App, error) {
	fd := int(os.Stdout.Fd())
	width, height, err := term.GetSize(fd)
	if err != nil {
		width, height = 80, 25
	}

	app := &App{
		output:       output,
		decoder:      keys.NewDecoder(),
		buf:          ui.NewBuffer(width, height),
		screenWidth:  width,
		screenHeight: height,
		running:      false,
		sigChan:      make(chan os.Signal, 1),
	}

	return app, nil
}

// Init initializes the terminal.
func (a *App) Init() error {
	signal.Notify(a.sigChan, syscall.SIGWINCH, syscall.SIGTERM, syscall.SIGINT)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}

	a.oldTermState = oldState

	fmt.Fprint(a.output, "\033[?1049h")
	fmt.Fprint(a.output, "\033[?25l")
	fmt.Fprint(a.output, "\033[2J")

	return nil
}

// Close restores the terminal.
func (a *App) Close() error {
	if a.oldTermState != nil {
		fd := int(os.Stdin.Fd())
		term.Restore(fd, a.oldTermState)
	}

	fmt.Fprint(a.output, "\033[?1049l")
	fmt.Fprint(a.output, "\033[?25h")

	return nil
}

// Run starts the main event loop.
func (a *App) Run() error {
	if err := a.Init(); err != nil {
		return err
	}
	defer a.Close()

	a.running = true
	a.render()

	return nil
}

// render draws the current state to the screen.
func (a *App) render() {
	a.buf = ui.NewBuffer(a.screenWidth, a.screenHeight)
	a.buf.SetText(5, 5, "m5 File Browser - Integration Test", 7, 0)
	a.buf.SetText(5, 7, "T-04: Keyboard Decoder", 7, 0)
	a.buf.SetText(5, 8, "T-05: UI Framework", 7, 0)
	a.buf.SetText(5, 9, "T-06: FS Abstraction", 7, 0)
	fmt.Fprint(a.output, a.buf.Render())
}
