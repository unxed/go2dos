module github.com/unxed/go2dos/cmd/go2dos-vtui

go 1.26.6

require (
	github.com/unxed/go2dos v0.0.0
	github.com/unxed/vtinput v0.1.10
	github.com/unxed/vtui v0.1.390
	golang.org/x/term v0.46.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20260211053922-3d992dae95d1 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.11.0-alpha.8 // indirect
	github.com/emmansun/base64 v0.9.0 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gg v0.52.5 // indirect
	github.com/gogpu/gogpu v0.53.0 // indirect
	github.com/gogpu/gpucontext v0.28.0 // indirect
	github.com/gogpu/gputypes v0.5.2 // indirect
	github.com/gogpu/naga v0.18.0 // indirect
	github.com/gogpu/wgpu v0.31.6 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/hajimehoshi/ebiten/v2 v2.10.0-alpha.13.0.20260811162617-464c2ddfc34c // indirect
	github.com/jezek/xgb v1.3.1 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/neurlang/wayland v0.4.4 // indirect
	github.com/neurlang/winc v0.1.2 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/soniakeys/quant v1.0.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/unxed/goclip v0.1.2 // indirect
	github.com/unxed/keytrans v0.1.35 // indirect
	github.com/unxed/kiwi-go v0.1.0 // indirect
	github.com/unxed/libwinescape v0.2.1 // indirect
	github.com/unxed/localecp v0.1.7 // indirect
	github.com/unxed/winkeys v0.1.1 // indirect
	github.com/unxed/xkb-go v0.1.9 // indirect
	github.com/yalue/native_endian v1.0.2 // indirect
	github.com/yuin/goldmark v1.8.6 // indirect
	github.com/zzl/go-win32api/v2 v2.1.0 // indirect
	golang.design/x/clipboard v0.7.0 // indirect
	golang.org/x/exp v0.0.0-20190731235908-ec7cb31e5a56 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/mobile v0.0.0-20230301163155-e0f57694e12c // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/unxed/go2dos => ../..

// Подмены vtui не наследуются потребителем: список как в unxed/f4/go.mod.
replace github.com/ebitengine/purego => github.com/unxed/pureffi v0.1.21

replace github.com/ebitengine/hideconsole => ./internal/hideconsole

replace github.com/neurlang/wayland => github.com/unxed/wayland v0.4.5-0.20260929195943-eab109b70429

replace github.com/go-webgpu/goffi => github.com/unxed/goffi v0.1.11
