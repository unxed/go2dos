package dos

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Trash (-trash DIR, T22): a file that DOS deletes is moved to DIR instead of being
// removed. It covers INT 21h/41h, INT 21h/7141h (with wildcards) and the shell's DEL.
// A name that is taken in DIR gets ".~1", ".~2", ... (nothing in DIR is overwritten).
// Every move is appended to DIR/go2dos-trash.log: time, the original host path, the
// name in DIR (tab-separated), so a file can be put back by hand. If the file cannot
// be moved, it is NOT deleted (the call fails with "access denied"). A file that is
// already inside DIR is really removed (emptying the trash from DOS works). Directories
// are not moved: DOS removes only empty ones.
//
// The built-in shell also saves a file that COPY or ">" is about to overwrite: a copy of
// the old contents goes to the trash first (saveOverwritten).

// saveOverwritten copies an existing regular file to the trash before the shell's COPY or
// redirection truncates it. Without a trash, or for a file in the trash, nothing is done.
func (f *fsys) saveOverwritten(host string) error {
	if f.trash == "" || f.inTrash(host) {
		return nil
	}
	if st, err := os.Stat(host); err != nil || !st.Mode().IsRegular() {
		return nil
	}
	return f.stash(host, false)
}

const trashLog = "go2dos-trash.log"

// remove deletes the host file: into the trash, or for real without one.
func (f *fsys) remove(host string) error {
	if f.trash == "" || f.inTrash(host) {
		return os.Remove(host)
	}
	return f.stash(host, true)
}

func (f *fsys) inTrash(host string) bool {
	rel, err := filepath.Rel(f.trash, host)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// stash puts the file in the trash: moved (delete) or copied (the original is about to be
// overwritten, and stays until then).
func (f *fsys) stash(host string, move bool) error {
	if err := os.MkdirAll(f.trash, 0o777); err != nil {
		return err
	}
	base := filepath.Base(host)
	dst := filepath.Join(f.trash, base)
	for n := 1; ; n++ {
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			break
		}
		if n > 9999 {
			return fmt.Errorf("trash: no free name for %s", base)
		}
		dst = filepath.Join(f.trash, fmt.Sprintf("%s.~%d", base, n))
	}
	if move {
		if err := os.Rename(host, dst); err != nil {
			// Another file system: copy, then remove the original.
			if err := copyFile(host, dst); err != nil {
				os.Remove(dst)
				return err
			}
			if err := os.Remove(host); err != nil {
				os.Remove(dst)
				return err
			}
		}
	} else if err := copyFile(host, dst); err != nil {
		os.Remove(dst)
		return err
	}
	if l, err := os.OpenFile(filepath.Join(f.trash, trashLog), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666); err == nil {
		fmt.Fprintf(l, "%s\t%s\t%s\n", time.Now().Format(time.RFC3339), host, filepath.Base(dst))
		l.Close()
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
