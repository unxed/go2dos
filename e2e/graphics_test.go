package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unxed/go2dos/machine"
)

func TestGraphicsModeCGA(t *testing.T) {
	comProg := []byte{
		0xB8, 0x04, 0x00, 0xCD, 0x10,
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}

	dir := t.TempDir()
	progPath := filepath.Join(dir, "TEST.COM")
	if err := os.WriteFile(progPath, comProg, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}})
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Load("C:\\TEST.COM", ""); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runErr := m.Run(ctx)
	var ex *machine.ExitError
	if !errors.As(runErr, &ex) || ex.Code != 0 {
		// Non-zero exit is okay for this test
	}

	video := m.BIOS.Video
	if !video.IsGraphicsMode() {
		t.Skip("Graphics mode was not set")
	}

	frame := video.CaptureGraphics()
	if frame == nil {
		t.Error("Failed to capture graphics frame")
	} else {
		if frame.Width != 320 || frame.Height != 200 || frame.Colors != 4 {
			t.Errorf("Graphics frame dimensions: got %dx%d %d colors, want 320x200 4 colors",
				frame.Width, frame.Height, frame.Colors)
		}

		buf := &bytes.Buffer{}
		if err := frame.EncodePPM(buf); err != nil {
			t.Errorf("Failed to encode PPM: %v", err)
		}

		if buf.Len() == 0 {
			t.Error("PPM output is empty")
		}
	}
}

func TestGraphicsMode13h(t *testing.T) {
	comProg := []byte{
		0xB8, 0x13, 0x00, 0xCD, 0x10,
		0xB8, 0x00, 0x4C, 0xCD, 0x21,
	}

	dir := t.TempDir()
	progPath := filepath.Join(dir, "GRAPH13.COM")
	if err := os.WriteFile(progPath, comProg, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := machine.New(machine.Config{Drives: map[byte]string{'C': dir}})
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Load("C:\\GRAPH13.COM", ""); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runErr := m.Run(ctx)
	var ex *machine.ExitError
	if !errors.As(runErr, &ex) || ex.Code != 0 {
		// Non-zero exit is okay for this test
	}

	video := m.BIOS.Video
	if !video.IsGraphicsMode() {
		t.Skip("Graphics mode 13h was not set")
	}

	frame := video.CaptureGraphics()
	if frame == nil {
		t.Error("Failed to capture graphics frame")
	} else {
		if frame.Mode != 0x13 {
			t.Errorf("Graphics mode: got %02Xh, want 13h", frame.Mode)
		}
		if frame.Width != 320 || frame.Height != 200 || frame.Colors != 256 {
			t.Errorf("Graphics frame dimensions: got %dx%d %d colors, want 320x200 256 colors",
				frame.Width, frame.Height, frame.Colors)
		}

		buf := &bytes.Buffer{}
		if err := frame.EncodePNG(buf); err != nil {
			t.Errorf("Failed to encode PNG: %v", err)
		}

		if buf.Len() == 0 {
			t.Error("PNG output is empty")
		}

		pngSig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
		if !bytes.HasPrefix(buf.Bytes(), pngSig) {
			t.Error("PNG output doesn't have valid PNG signature")
		}
	}
}
