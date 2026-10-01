package dos

import (
	"path/filepath"
	"strings"
)

// Read-only drives (-ro C,D or -ro all): every change of the host file system that DOS
// makes goes through a check of wp. A write-protected drive answers "access denied"
// (error 5) to creating, truncating, writing, deleting, renaming, changing attributes
// and times. Host commands (HOSTEXEC) are not limited: they leave the sandbox anyway.

// wp reports whether the host path is on a read-only drive.
func (f *fsys) wp(host string) bool {
	for l, root := range f.drives {
		if root == "" || !f.ro[l] {
			continue
		}
		if rel, err := filepath.Rel(root, host); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ParseReadOnly turns the value of -ro ("C", "CD", "C,D", "all") into drive letters.
func ParseReadOnly(s string) (map[byte]bool, error) {
	out := map[byte]bool{}
	if strings.EqualFold(strings.TrimSpace(s), "all") {
		for l := byte('A'); l <= 'Z'; l++ {
			out[l] = true
		}
		return out, nil
	}
	for _, c := range strings.ToUpper(s) {
		switch {
		case c == ',' || c == ' ' || c == ':':
		case c >= 'A' && c <= 'Z':
			out[byte(c)] = true
		default:
			return nil, errBadRO(s)
		}
	}
	return out, nil
}

type errBadRO string

func (e errBadRO) Error() string {
	return "-ro: want drive letters (C or CD or C,D) or all, got " + string(e)
}

// insideRoot tells whether the symbolic link dir/name leads to a place below the root of
// the drive that dir belongs to (-confine). A link that cannot be followed is outside.
func (f *fsys) insideRoot(dir, name string) bool {
	real, err := filepath.EvalSymlinks(filepath.Join(dir, name))
	if err != nil {
		return false
	}
	for l, root := range f.drives {
		if root == "" {
			continue
		}
		if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if f.realRoot[l] == "" {
			if f.realRoot[l], err = filepath.EvalSymlinks(root); err != nil {
				f.realRoot[l] = root
			}
		}
		rel, err := filepath.Rel(f.realRoot[l], real)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	return false
}
