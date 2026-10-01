// Package hideconsole заменяет github.com/ebitengine/hideconsole: vtui
// требует эту зависимость, а настоящая прячет консоль Windows, что терминальному
// фронтенду не нужно (так же сделано в unxed/vtui и unxed/f4).
package hideconsole

// Hide ничего не делает.
func Hide() error { return nil }
