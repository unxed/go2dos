package e2e

import (
	"testing"

	"github.com/unxed/go2dos/frontend"
)

// TestMemoryClipboardFiles tests file operations on in-memory clipboard.
func TestMemoryClipboardFiles(t *testing.T) {
	mc := &frontend.MemoryClipboard{}

	testPaths := []string{
		"C:\FILE1.TXT",
		"C:\FILE2.DOC",
		"D:\SUBDIR\FILE3.XLS",
	}

	if err := mc.SetFiles(testPaths); err != nil {
		t.Fatalf("SetFiles failed: %v", err)
	}

	paths, err := mc.GetFiles()
	if err != nil {
		t.Fatalf("GetFiles failed: %v", err)
	}

	if len(paths) != len(testPaths) {
		t.Errorf("Expected %d files, got %d", len(testPaths), len(paths))
	}

	for i, p := range paths {
		if p != testPaths[i] {
			t.Errorf("File %d: expected %q, got %q", i, testPaths[i], p)
		}
	}
}

// TestMemoryClipboardCoexistence tests that text and files can coexist.
func TestMemoryClipboardCoexistence(t *testing.T) {
	mc := &frontend.MemoryClipboard{}

	testText := "Hello, World!"
	testPaths := []string{"C:\FILE.TXT"}

	if err := mc.SetText(testText); err != nil {
		t.Fatalf("SetText failed: %v", err)
	}

	if err := mc.SetFiles(testPaths); err != nil {
		t.Fatalf("SetFiles failed: %v", err)
	}

	text, err := mc.GetText()
	if err != nil {
		t.Fatalf("GetText failed: %v", err)
	}

	if text != testText {
		t.Errorf("Text mismatch: expected %q, got %q", testText, text)
	}

	paths, err := mc.GetFiles()
	if err != nil {
		t.Fatalf("GetFiles failed: %v", err)
	}

	if len(paths) != 1 || paths[0] != testPaths[0] {
		t.Errorf("Files mismatch: expected %v, got %v", testPaths, paths)
	}
}

// TestCFHDROPEncoding tests CF_HDROP format encoding and decoding.
func TestCFHDROPEncoding(t *testing.T) {
	testCases := []struct {
		name  string
		paths []string
	}{
		{
			name:  "single file",
			paths: []string{"C:\FILE.TXT"},
		},
		{
			name: "multiple files",
			paths: []string{
				"C:\FILE1.TXT",
				"D:\SUBDIR\FILE2.DOC",
				"E:\FILE3.XLS",
			},
		},
		{
			name:  "file with spaces",
			paths: []string{"C:\My Documents\My File.txt"},
		},
		{
			name:  "empty list",
			paths: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := frontend.EncodeCFHDROP(tc.paths)

			if len(tc.paths) == 0 {
				if len(encoded) != 4 {
					t.Errorf("Empty HDROP should be 4 bytes, got %d", len(encoded))
				}
				return
			}

			decoded, err := frontend.DecodeCFHDROP(encoded)
			if err != nil {
				t.Fatalf("DecodeCFHDROP failed: %v", err)
			}

			if len(decoded) != len(tc.paths) {
				t.Errorf("Expected %d files, got %d", len(tc.paths), len(decoded))
			}

			for i, p := range decoded {
				if p != tc.paths[i] {
					t.Errorf("File %d: expected %q, got %q", i, tc.paths[i], p)
				}
			}
		})
	}
}

// TestFilePathEncoding tests various file path encodings in HDROP.
func TestFilePathEncoding(t *testing.T) {
	testCases := []struct {
		name  string
		paths []string
	}{
		{
			name:  "DOS paths",
			paths: []string{"C:\DIR\FILE.TXT", "D:\DATA\TEST.DAT"},
		},
		{
			name:  "UNC paths",
			paths: []string{"\\SERVER\SHARE\FILE.TXT"},
		},
		{
			name:  "long paths",
			paths: []string{"C:\VERY\LONG\DIRECTORY\PATH\TO\SOME\FILE.TXT"},
		},
		{
			name: "mixed paths",
			paths: []string{
				"C:\SHORT.TXT",
				"C:\LONGER\PATH\FILE.DOC",
				"\\NETWORK\SHARE\FILE.XLS",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := frontend.EncodeCFHDROP(tc.paths)
			decoded, err := frontend.DecodeCFHDROP(encoded)
			if err != nil {
				t.Fatalf("DecodeCFHDROP failed: %v", err)
			}

			if len(decoded) != len(tc.paths) {
				t.Fatalf("Path count mismatch: expected %d, got %d", len(tc.paths), len(decoded))
			}

			for i, p := range decoded {
				if p != tc.paths[i] {
					t.Errorf("Path %d: expected %q, got %q", i, tc.paths[i], p)
				}
			}
		})
	}
}
