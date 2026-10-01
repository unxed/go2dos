package bios

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

type GraphicsMode struct {
	Width      int
	Height     int
	Colors     int
	BitsPerPix int
	MemStart   uint32
	MemSize    uint32
}

func SupportedGraphicsModes() map[byte]GraphicsMode {
	return map[byte]GraphicsMode{
		0x04: {Width: 320, Height: 200, Colors: 4, BitsPerPix: 2, MemStart: 0xB8000, MemSize: 0x4000},
		0x05: {Width: 320, Height: 200, Colors: 4, BitsPerPix: 2, MemStart: 0xB8000, MemSize: 0x4000},
		0x06: {Width: 640, Height: 200, Colors: 2, BitsPerPix: 1, MemStart: 0xB8000, MemSize: 0x4000},
		0x0F: {Width: 720, Height: 348, Colors: 2, BitsPerPix: 1, MemStart: 0xB0000, MemSize: 0x7C00},
		0x11: {Width: 640, Height: 480, Colors: 2, BitsPerPix: 1, MemStart: 0xA0000, MemSize: 0x9600},
		0x12: {Width: 640, Height: 480, Colors: 16, BitsPerPix: 4, MemStart: 0xA0000, MemSize: 0x25800},
		0x13: {Width: 320, Height: 200, Colors: 256, BitsPerPix: 8, MemStart: 0xA0000, MemSize: 0x10000},
	}
}

type GraphicsFrame struct {
	Mode    byte
	Width   int
	Height  int
	Colors  int
	Data    []byte
	Palette [256]color.Color
	Version uint32
}

func (v *Video) captureGraphics() *GraphicsFrame {
	mode := v.mode()
	modes := SupportedGraphicsModes()
	gm, ok := modes[mode]
	if !ok {
		return nil
	}

	data := make([]byte, gm.MemSize)
	for i := uint32(0); i < gm.MemSize; i++ {
		data[i] = v.b8(gm.MemStart + i)
	}

	frame := &GraphicsFrame{
		Mode:    mode,
		Width:   gm.Width,
		Height:  gm.Height,
		Colors:  gm.Colors,
		Data:    data,
		Version: v.e.Mem.RangeWrites(gm.MemStart, gm.MemStart+gm.MemSize) + v.crtcGen,
	}

	frame.setPaletteDefaults()

	return frame
}

func (f *GraphicsFrame) setPaletteDefaults() {
	switch f.Mode {
	case 0x04, 0x05:
		f.Palette[0] = color.RGBA{0x00, 0x00, 0x00, 0xFF}
		f.Palette[1] = color.RGBA{0x00, 0xAA, 0xAA, 0xFF}
		f.Palette[2] = color.RGBA{0xAA, 0x00, 0xAA, 0xFF}
		f.Palette[3] = color.RGBA{0xAA, 0xAA, 0xAA, 0xFF}
	case 0x06, 0x0F, 0x11:
		// Monochrome modes: black and white
		f.Palette[0] = color.RGBA{0x00, 0x00, 0x00, 0xFF}
		f.Palette[1] = color.RGBA{0xAA, 0xAA, 0xAA, 0xFF}
	case 0x12:
		// VGA 16-color mode
		setPalette16Default(f.Palette[:])
	case 0x13:
		setPalette256Default(f.Palette[:])
	}
}

func setPalette16Default(palette []color.Color) {
	// Standard 16-color VGA palette
	colors := [16]color.RGBA{
		{0x00, 0x00, 0x00, 0xFF}, // Black
		{0x00, 0x00, 0xAA, 0xFF}, // Blue
		{0x00, 0xAA, 0x00, 0xFF}, // Green
		{0x00, 0xAA, 0xAA, 0xFF}, // Cyan
		{0xAA, 0x00, 0x00, 0xFF}, // Red
		{0xAA, 0x00, 0xAA, 0xFF}, // Magenta
		{0xAA, 0x55, 0x00, 0xFF}, // Brown
		{0xAA, 0xAA, 0xAA, 0xFF}, // Light gray
		{0x55, 0x55, 0x55, 0xFF}, // Dark gray
		{0x55, 0x55, 0xFF, 0xFF}, // Light blue
		{0x55, 0xFF, 0x55, 0xFF}, // Light green
		{0x55, 0xFF, 0xFF, 0xFF}, // Light cyan
		{0xFF, 0x55, 0x55, 0xFF}, // Light red
		{0xFF, 0x55, 0xFF, 0xFF}, // Light magenta
		{0xFF, 0xFF, 0x55, 0xFF}, // Yellow
		{0xFF, 0xFF, 0xFF, 0xFF}, // White
	}
	for i, c := range colors {
		palette[i] = c
	}
}

func setPalette256Default(palette []color.Color) {
	egaPalette := [16]color.RGBA{
		{0x00, 0x00, 0x00, 0xFF},
		{0x00, 0x00, 0xAA, 0xFF},
		{0x00, 0xAA, 0x00, 0xFF},
		{0x00, 0xAA, 0xAA, 0xFF},
		{0xAA, 0x00, 0x00, 0xFF},
		{0xAA, 0x00, 0xAA, 0xFF},
		{0xAA, 0x55, 0x00, 0xFF},
		{0xAA, 0xAA, 0xAA, 0xFF},
		{0x55, 0x55, 0x55, 0xFF},
		{0x55, 0x55, 0xFF, 0xFF},
		{0x55, 0xFF, 0x55, 0xFF},
		{0x55, 0xFF, 0xFF, 0xFF},
		{0xFF, 0x55, 0x55, 0xFF},
		{0xFF, 0x55, 0xFF, 0xFF},
		{0xFF, 0xFF, 0x55, 0xFF},
		{0xFF, 0xFF, 0xFF, 0xFF},
	}
	for i, c := range egaPalette {
		palette[i] = c
	}

	for i := 0; i < 216; i++ {
		r := (i / 36) * 51
		g := ((i / 6) % 6) * 51
		b := (i % 6) * 51
		palette[16+i] = color.RGBA{uint8(r), uint8(g), uint8(b), 0xFF}
	}

	for i := 0; i < 24; i++ {
		gray := uint8(8 + i*10)
		palette[232+i] = color.RGBA{gray, gray, gray, 0xFF}
	}
}

func (f *GraphicsFrame) ToImage() image.Image {
	rect := image.Rect(0, 0, f.Width, f.Height)
	img := image.NewRGBA(rect)

	switch f.Mode {
	case 0x04, 0x05:
		f.decodeModeCGA(img)
	case 0x06:
		f.decodeModeMonochrome(img)
	case 0x0F, 0x11:
		// Monochrome modes (Hercules and VGA mono)
		f.decodeModeMonochrome(img)
	case 0x12:
		f.decodeModeVGA16(img)
	case 0x13:
		f.decodeModeVGA256(img)
	}

	return img
}

func (f *GraphicsFrame) decodeModeCGA(img *image.RGBA) {
	for row := 0; row < f.Height; row++ {
		base := uint32(0)
		if row%2 == 1 {
			base = 0x2000
		}
		lineOffset := base + uint32((row/2)*f.Width/4)

		for col := 0; col < f.Width; col++ {
			byteIdx := lineOffset + uint32(col/4)
			if byteIdx >= uint32(len(f.Data)) {
				continue
			}

			byte_ := f.Data[byteIdx]
			pixelInByte := col % 4
			colorIdx := (byte_ >> uint(6-pixelInByte*2)) & 0x03

			c := f.Palette[colorIdx]
			img.SetRGBA(col, row, c.(color.RGBA))
		}
	}
}

func (f *GraphicsFrame) decodeModeMonochrome(img *image.RGBA) {
	for row := 0; row < f.Height; row++ {
		lineOffset := uint32(row * f.Width / 8)

		for col := 0; col < f.Width; col++ {
			byteIdx := lineOffset + uint32(col/8)
			if byteIdx >= uint32(len(f.Data)) {
				continue
			}

			byte_ := f.Data[byteIdx]
			pixelInByte := col % 8
			colorIdx := (byte_ >> uint(7-pixelInByte)) & 0x01

			c := f.Palette[colorIdx]
			img.SetRGBA(col, row, c.(color.RGBA))
		}
	}
}

func (f *GraphicsFrame) decodeModeVGA16(img *image.RGBA) {
	// VGA mode 12h: 640x480 16-color planar mode
	// Each pixel uses 4 bits (one from each plane)
	// Memory layout: 4 planes of 38400 bytes each
	planeSize := uint32(640 * 480 / 8) // 0x9600 bytes per plane

	for row := 0; row < f.Height; row++ {
		lineOffset := uint32((row * f.Width) / 8)

		for col := 0; col < f.Width; col++ {
			byteIdx := lineOffset + uint32(col/8)
			if byteIdx >= uint32(len(f.Data)) {
				continue
			}

			bitInByte := 7 - (col % 8)

			// Extract color index from 4 planes
			colorIdx := uint8(0)
			for plane := 0; plane < 4; plane++ {
				planeOffset := byteIdx + uint32(plane)*planeSize
				if planeOffset >= uint32(len(f.Data)) {
					continue
				}
				bit := (f.Data[planeOffset] >> uint(bitInByte)) & 0x01
				colorIdx |= bit << uint(plane)
			}

			c := f.Palette[colorIdx]
			img.SetRGBA(col, row, c.(color.RGBA))
		}
	}
}

func (f *GraphicsFrame) decodeModeVGA256(img *image.RGBA) {
	for row := 0; row < f.Height; row++ {
		lineOffset := uint32(row * f.Width)

		for col := 0; col < f.Width; col++ {
			byteIdx := lineOffset + uint32(col)
			if byteIdx >= uint32(len(f.Data)) {
				continue
			}

			colorIdx := f.Data[byteIdx]
			c := f.Palette[colorIdx]
			img.SetRGBA(col, row, c.(color.RGBA))
		}
	}
}

func (f *GraphicsFrame) EncodePPM(w io.Writer) error {
	img := f.ToImage()

	header := fmt.Sprintf("P6\n%d %d\n255\n", f.Width, f.Height)
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			pixel := []byte{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
			if _, err := w.Write(pixel); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *GraphicsFrame) EncodePNG(w io.Writer) error {
	img := f.ToImage()
	return png.Encode(w, img)
}

func (f *GraphicsFrame) EncodeASCIIArt(w io.Writer) error {
	img := f.ToImage()
	bounds := img.Bounds()

	scale := 1
	if f.Height > 50 {
		scale = f.Height / 25
	}
	if f.Width > 160 {
		scale = (f.Width / 80)
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += scale {
		for x := bounds.Min.X; x < bounds.Max.X; x += scale {
			r, g, b, _ := img.At(x, y).RGBA()
			brightness := (r + g + b) / 3
			char := ' '
			switch brightness >> 12 {
			case 0, 1, 2, 3:
				char = ' '
			case 4, 5, 6, 7:
				char = '.'
			case 8, 9, 10, 11:
				char = 'o'
			case 12, 13, 14, 15:
				char = '#'
			}
			w.Write([]byte{byte(char)})
		}
		w.Write([]byte{'\n'})
	}

	return nil
}

func (f *GraphicsFrame) String() string {
	return fmt.Sprintf("Graphics Mode %02Xh: %dx%d, %d colors, version %d",
		f.Mode, f.Width, f.Height, f.Colors, f.Version)
}

func (v *Video) CaptureGraphics() *GraphicsFrame {
	return v.captureGraphics()
}
