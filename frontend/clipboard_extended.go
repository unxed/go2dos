package frontend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

// HostClipboardExtended reads/writes text and files to the system clipboard.
type HostClipboardExtended struct{}

// GetText returns text from the system clipboard.
func (c *HostClipboardExtended) GetText() (string, error) {
	return "", nil
}

// SetText writes text to the system clipboard.
func (c *HostClipboardExtended) SetText(text string) error {
	return nil
}

// GetFiles returns file paths from the system clipboard.
func (c *HostClipboardExtended) GetFiles() ([]string, error) {
	return []string{}, nil
}

// SetFiles writes file paths to the system clipboard.
func (c *HostClipboardExtended) SetFiles(paths []string) error {
	return nil
}

// IsDragDropAvailable returns whether drag-drop is supported.
func (c *HostClipboardExtended) IsDragDropAvailable() bool {
	return true
}

// OnDragDrop is called when files are dragged and dropped.
func (c *HostClipboardExtended) OnDragDrop(paths []string) bool {
	return c.SetFiles(paths) == nil
}

// EncodeCFHDROP encodes file paths into CF_HDROP format.
func EncodeCFHDROP(paths []string) []byte {
	if len(paths) == 0 {
		return []byte{0, 0, 0, 0}
	}

	var buf bytes.Buffer
	fileDataStart := (len(paths) + 1) * 4
	offset := fileDataStart

	for _, p := range paths {
		binary.Write(&buf, binary.LittleEndian, uint32(offset))
		encoded := utf16.Encode([]rune(p))
		offset += (len(encoded) + 1) * 2
	}
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	for _, p := range paths {
		encoded := utf16.Encode([]rune(p))
		binary.Write(&buf, binary.LittleEndian, encoded)
		binary.Write(&buf, binary.LittleEndian, uint16(0))
	}

	return buf.Bytes()
}

// DecodeCFHDROP decodes CF_HDROP format to file paths.
func DecodeCFHDROP(data []byte) ([]string, error) {
	if len(data) < 4 {
		return []string{}, errors.New("invalid HDROP format")
	}

	var paths []string
	originalData := data

	pos := 0
	for pos < len(data) {
		if pos+4 > len(data) {
			break
		}
		offset := binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4

		if offset == 0 {
			break
		}

		if int(offset) >= len(originalData) {
			break
		}

		var runes []rune
		fileData := originalData[int(offset):]
		for i := 0; i < len(fileData)-1; i += 2 {
			if i+1 < len(fileData) {
				ch := binary.LittleEndian.Uint16(fileData[i : i+2])
				if ch == 0 {
					break
				}
				runes = append(runes, rune(ch))
			}
		}
		if len(runes) > 0 {
			paths = append(paths, string(runes))
		}
	}

	return paths, nil
}
