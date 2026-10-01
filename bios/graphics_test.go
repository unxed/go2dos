package bios

import (
	"bytes"
	"image/png"
	"testing"
)

func TestSupportedGraphicsModes(t *testing.T) {
	modes := SupportedGraphicsModes()

	expectedModes := map[byte]struct {
		width, height, colors int
	}{
		0x04: {320, 200, 4},
		0x05: {320, 200, 4},
		0x06: {640, 200, 2},
		0x13: {320, 200, 256},
	}

	for mode, expected := range expectedModes {
		gm, ok := modes[mode]
		if !ok {
			t.Errorf("Graphics mode %02Xh not defined", mode)
			continue
		}
		if gm.Width != expected.width || gm.Height != expected.height || gm.Colors != expected.colors {
			t.Errorf("Mode %02Xh: got %dx%d %d colors, want %dx%d %d colors",
				mode, gm.Width, gm.Height, gm.Colors, expected.width, expected.height, expected.colors)
		}
	}
}

func TestGraphicsFrameCreation(t *testing.T) {
	frame := &GraphicsFrame{
		Mode:    0x13,
		Width:   320,
		Height:  200,
		Colors:  256,
		Data:    make([]byte, 320*200),
		Version: 42,
	}
	frame.setPaletteDefaults()

	if frame.String() == "" {
		t.Error("Frame.String() returned empty string")
	}

	if frame.Palette[0] == nil {
		t.Error("Palette colors not initialized")
	}
}

func TestGraphicsFrameToImageVGA256(t *testing.T) {
	data := make([]byte, 320*200)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i % 256)
	}

	frame := &GraphicsFrame{
		Mode:   0x13,
		Width:  320,
		Height: 200,
		Colors: 256,
		Data:   data,
	}
	frame.setPaletteDefaults()

	img := frame.ToImage()

	if img.Bounds().Max.X != 320 || img.Bounds().Max.Y != 200 {
		t.Errorf("Image size: got %dx%d, want 320x200", img.Bounds().Max.X, img.Bounds().Max.Y)
	}
}

func TestEncodePPM(t *testing.T) {
	frame := &GraphicsFrame{
		Mode:   0x13,
		Width:  2,
		Height: 2,
		Colors: 256,
		Data:   []byte{0, 1, 2, 3},
	}
	frame.setPaletteDefaults()

	buf := &bytes.Buffer{}
	if err := frame.EncodePPM(buf); err != nil {
		t.Fatalf("EncodePPM failed: %v", err)
	}

	if !bytes.HasPrefix(buf.Bytes(), []byte("P6\n")) {
		t.Error("PPM output doesn't start with magic number")
	}

	if !bytes.Contains(buf.Bytes(), []byte("2 2")) {
		t.Error("PPM header doesn't contain dimensions")
	}
}

func TestEncodePNG(t *testing.T) {
	frame := &GraphicsFrame{
		Mode:   0x13,
		Width:  8,
		Height: 8,
		Colors: 256,
		Data:   make([]byte, 64),
	}
	frame.setPaletteDefaults()

	buf := &bytes.Buffer{}
	if err := frame.EncodePNG(buf); err != nil {
		t.Fatalf("EncodePNG failed: %v", err)
	}

	pngSig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(buf.Bytes(), pngSig) {
		t.Error("PNG output doesn't have valid PNG signature")
	}

	_, err := png.DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Errorf("Generated PNG is not valid: %v", err)
	}
}

func TestEncodeASCIIArt(t *testing.T) {
	frame := &GraphicsFrame{
		Mode:   0x13,
		Width:  16,
		Height: 16,
		Colors: 256,
		Data:   make([]byte, 256),
	}
	frame.setPaletteDefaults()

	buf := &bytes.Buffer{}
	if err := frame.EncodeASCIIArt(buf); err != nil {
		t.Fatalf("EncodeASCIIArt failed: %v", err)
	}

	if buf.String() == "" {
		t.Error("ASCII art output is empty")
	}

	if !bytes.Contains(buf.Bytes(), []byte("\n")) {
		t.Error("ASCII art output doesn't contain newlines")
	}
}

func TestCGAPaletteDefaults(t *testing.T) {
	frame := &GraphicsFrame{Mode: 0x04, Colors: 4}
	frame.setPaletteDefaults()

	if len(frame.Palette) != 256 {
		t.Errorf("Palette should have 256 entries, got %d", len(frame.Palette))
	}

	r, g, b, _ := frame.Palette[0].RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("Color 0 should be black, got RGBA(%d, %d, %d)", r, g, b)
	}
}

func TestVGA256PaletteDefaults(t *testing.T) {
	frame := &GraphicsFrame{Mode: 0x13, Colors: 256}
	frame.setPaletteDefaults()

	r, g, b, _ := frame.Palette[0].RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("Color 0 should be black, got RGBA(%d, %d, %d)", r, g, b)
	}

	if frame.Palette[255] == nil {
		t.Error("Palette[255] not initialized")
	}
}
