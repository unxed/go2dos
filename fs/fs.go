// Package fs provides a filesystem abstraction layer independent of the OS.
package fs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileSystem is an abstraction for working with files independent of the OS.
type FileSystem interface {
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	Remove(path string) error
	RemoveAll(path string) error
	Rename(oldpath, newpath string) error
	Mkdir(name string, perm iofs.FileMode) error
	MkdirAll(path string, perm iofs.FileMode) error
	Stat(path string) (iofs.FileInfo, error)
	Lstat(path string) (iofs.FileInfo, error)
	ReadDir(path string) ([]iofs.DirEntry, error)
	Exists(path string) bool
	IsDir(path string) bool
	Walk(root string, fn func(path string, info interface{}, err error) error) error
}

// OSFileSystem implements FileSystem using the operating system's filesystem.
type OSFileSystem struct{}

func NewOSFileSystem() FileSystem {
	return &OSFileSystem{}
}

func (osfs *OSFileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (osfs *OSFileSystem) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

func (osfs *OSFileSystem) Remove(path string) error {
	return os.Remove(path)
}

func (osfs *OSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (osfs *OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osfs *OSFileSystem) Mkdir(name string, perm iofs.FileMode) error {
	return os.Mkdir(name, os.FileMode(perm))
}

func (osfs *OSFileSystem) MkdirAll(path string, perm iofs.FileMode) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

func (osfs *OSFileSystem) Stat(path string) (iofs.FileInfo, error) {
	return os.Stat(path)
}

func (osfs *OSFileSystem) Lstat(path string) (iofs.FileInfo, error) {
	return os.Lstat(path)
}

func (osfs *OSFileSystem) ReadDir(path string) ([]iofs.DirEntry, error) {
	return os.ReadDir(path)
}

func (osfs *OSFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (osfs *OSFileSystem) IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (osfs *OSFileSystem) Walk(root string, fn func(path string, info interface{}, err error) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		return fn(path, info, err)
	})
}

// MemoryFileSystem implements FileSystem using in-memory storage.
type MemoryFileSystem struct {
	root *memFile
}

type memFile struct {
	name     string
	isDir    bool
	mode     iofs.FileMode
	modTime  time.Time
	data     *bytes.Buffer
	children map[string]*memFile
	parent   *memFile
}

func NewMemoryFileSystem() FileSystem {
	root := &memFile{
		name:     "/",
		isDir:    true,
		mode:     iofs.ModeDir | 0o755,
		modTime:  time.Now(),
		children: make(map[string]*memFile),
	}
	return &MemoryFileSystem{root: root}
}

func (vfs *MemoryFileSystem) Open(path string) (io.ReadCloser, error) {
	vf, err := vfs.findFile(path)
	if err != nil {
		return nil, err
	}
	if vf.isDir {
		return nil, errors.New("is a directory")
	}
	return io.NopCloser(bytes.NewReader(vf.data.Bytes())), nil
}

func (vfs *MemoryFileSystem) Create(path string) (io.WriteCloser, error) {
	parent, name := vfs.split(path)
	parentFile, err := vfs.findFile(parent)
	if err != nil {
		return nil, err
	}
	if !parentFile.isDir {
		return nil, errors.New("parent is not a directory")
	}

	vf := &memFile{
		name:    name,
		isDir:   false,
		mode:    0o644,
		modTime: time.Now(),
		data:    &bytes.Buffer{},
	}
	parentFile.children[name] = vf
	vf.parent = parentFile

	return nopCloser{vf.data}, nil
}

func (vfs *MemoryFileSystem) Remove(path string) error {
	parent, name := vfs.split(path)
	parentFile, err := vfs.findFile(parent)
	if err != nil {
		return err
	}
	vf, ok := parentFile.children[name]
	if !ok {
		return os.ErrNotExist
	}
	if vf.isDir && len(vf.children) > 0 {
		return errors.New("directory not empty")
	}
	delete(parentFile.children, name)
	return nil
}

func (vfs *MemoryFileSystem) RemoveAll(path string) error {
	parent, name := vfs.split(path)
	parentFile, err := vfs.findFile(parent)
	if err != nil {
		return err
	}
	delete(parentFile.children, name)
	return nil
}

func (vfs *MemoryFileSystem) Rename(oldpath, newpath string) error {
	oldParent, oldName := vfs.split(oldpath)
	newParent, newName := vfs.split(newpath)

	oldParentFile, err := vfs.findFile(oldParent)
	if err != nil {
		return err
	}
	newParentFile, err := vfs.findFile(newParent)
	if err != nil {
		return err
	}

	vf, ok := oldParentFile.children[oldName]
	if !ok {
		return os.ErrNotExist
	}

	delete(oldParentFile.children, oldName)
	vf.name = newName
	newParentFile.children[newName] = vf
	vf.parent = newParentFile

	return nil
}

func (vfs *MemoryFileSystem) Mkdir(name string, perm iofs.FileMode) error {
	parent, dirName := vfs.split(name)
	parentFile, err := vfs.findFile(parent)
	if err != nil {
		return err
	}

	if _, ok := parentFile.children[dirName]; ok {
		return os.ErrExist
	}

	vf := &memFile{
		name:     dirName,
		isDir:    true,
		mode:     iofs.ModeDir | perm,
		modTime:  time.Now(),
		children: make(map[string]*memFile),
	}
	parentFile.children[dirName] = vf
	vf.parent = parentFile

	return nil
}

func (vfs *MemoryFileSystem) MkdirAll(path string, perm iofs.FileMode) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = "/" + part
		} else {
			current = current + "/" + part
		}
		if !vfs.IsDir(current) {
			if err := vfs.Mkdir(current, perm); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
		}
	}
	return nil
}

func (vfs *MemoryFileSystem) Stat(path string) (iofs.FileInfo, error) {
	vf, err := vfs.findFile(path)
	if err != nil {
		return nil, err
	}
	return &memFileInfo{vf}, nil
}

func (vfs *MemoryFileSystem) Lstat(path string) (iofs.FileInfo, error) {
	return vfs.Stat(path)
}

func (vfs *MemoryFileSystem) ReadDir(path string) ([]iofs.DirEntry, error) {
	vf, err := vfs.findFile(path)
	if err != nil {
		return nil, err
	}
	if !vf.isDir {
		return nil, errors.New("not a directory")
	}

	var entries []iofs.DirEntry
	names := make([]string, 0, len(vf.children))
	for name := range vf.children {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entries = append(entries, &memDirEntry{vf.children[name]})
	}

	return entries, nil
}

func (vfs *MemoryFileSystem) Exists(path string) bool {
	_, err := vfs.findFile(path)
	return err == nil
}

func (vfs *MemoryFileSystem) IsDir(path string) bool {
	vf, err := vfs.findFile(path)
	return err == nil && vf.isDir
}

func (vfs *MemoryFileSystem) Walk(root string, fn func(path string, info interface{}, err error) error) error {
	return vfs.walk(root, fn)
}

func (vfs *MemoryFileSystem) walk(path string, fn func(path string, info interface{}, err error) error) error {
	vf, err := vfs.findFile(path)
	if err != nil {
		return fn(path, nil, err)
	}

	info := &memFileInfo{vf}
	if err := fn(path, info, nil); err != nil {
		return err
	}

	if !vf.isDir {
		return nil
	}

	names := make([]string, 0, len(vf.children))
	for name := range vf.children {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		childPath := path + "/" + name
		if err := vfs.walk(childPath, fn); err != nil {
			return err
		}
	}

	return nil
}

func (vfs *MemoryFileSystem) findFile(path string) (*memFile, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return vfs.root, nil
	}

	parts := strings.Split(path, "/")
	current := vfs.root

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if current.parent != nil {
				current = current.parent
			}
			continue
		}

		next, ok := current.children[part]
		if !ok {
			return nil, fmt.Errorf("not found: %s", path)
		}
		current = next
	}

	return current, nil
}

func (vfs *MemoryFileSystem) split(path string) (string, string) {
	path = strings.TrimSuffix(path, "/")
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return "/", path
	}
	if idx == 0 {
		return "/", path[1:]
	}
	return path[:idx], path[idx+1:]
}

type memFileInfo struct {
	file *memFile
}

func (fi *memFileInfo) Name() string {
	return fi.file.name
}

func (fi *memFileInfo) Size() int64 {
	if fi.file.data != nil {
		return int64(fi.file.data.Len())
	}
	return 0
}

func (fi *memFileInfo) Mode() iofs.FileMode {
	return fi.file.mode
}

func (fi *memFileInfo) ModTime() time.Time {
	return fi.file.modTime
}

func (fi *memFileInfo) IsDir() bool {
	return fi.file.isDir
}

func (fi *memFileInfo) Sys() interface{} {
	return nil
}

type memDirEntry struct {
	file *memFile
}

func (de *memDirEntry) Name() string {
	return de.file.name
}

func (de *memDirEntry) IsDir() bool {
	return de.file.isDir
}

func (de *memDirEntry) Type() iofs.FileMode {
	if de.file.isDir {
		return iofs.ModeDir
	}
	return 0
}

func (de *memDirEntry) Info() (iofs.FileInfo, error) {
	return &memFileInfo{de.file}, nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

// DOSFileSystem adapts DOS-style paths to the FileSystem interface.
type DOSFileSystem struct {
	rootPath string
}

func NewDOSFileSystem(rootPath string) (FileSystem, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("rootPath is not a directory")
	}

	abs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	return &DOSFileSystem{rootPath: abs}, nil
}

func (dfs *DOSFileSystem) convertPath(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		path = path[2:]
	}
	path = strings.ReplaceAll(path, `\`, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return filepath.Join(dfs.rootPath, path)
}

func (dfs *DOSFileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) Create(path string) (io.WriteCloser, error) {
	return os.Create(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) Remove(path string) error {
	return os.Remove(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(dfs.convertPath(oldpath), dfs.convertPath(newpath))
}

func (dfs *DOSFileSystem) Mkdir(name string, perm iofs.FileMode) error {
	return os.Mkdir(dfs.convertPath(name), os.FileMode(perm))
}

func (dfs *DOSFileSystem) MkdirAll(path string, perm iofs.FileMode) error {
	return os.MkdirAll(dfs.convertPath(path), os.FileMode(perm))
}

func (dfs *DOSFileSystem) Stat(path string) (iofs.FileInfo, error) {
	return os.Stat(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) Lstat(path string) (iofs.FileInfo, error) {
	return os.Lstat(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) ReadDir(path string) ([]iofs.DirEntry, error) {
	return os.ReadDir(dfs.convertPath(path))
}

func (dfs *DOSFileSystem) Exists(path string) bool {
	_, err := os.Stat(dfs.convertPath(path))
	return err == nil
}

func (dfs *DOSFileSystem) IsDir(path string) bool {
	info, err := os.Stat(dfs.convertPath(path))
	return err == nil && info.IsDir()
}

func (dfs *DOSFileSystem) Walk(root string, fn func(path string, info interface{}, err error) error) error {
	return filepath.Walk(dfs.convertPath(root), func(path string, info os.FileInfo, err error) error {
		return fn(path, info, err)
	})
}

// DOSMemoryFileSystem implements FileSystem using in-memory storage with DOS paths.
type DOSMemoryFileSystem struct {
	vfs *MemoryFileSystem
}

func NewDOSMemoryFileSystem() FileSystem {
	return &DOSMemoryFileSystem{
		vfs: NewMemoryFileSystem().(*MemoryFileSystem),
	}
}

func (dmfs *DOSMemoryFileSystem) convertPath(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		path = path[2:]
	}
	path = strings.ReplaceAll(path, `\`, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func (dmfs *DOSMemoryFileSystem) Open(path string) (io.ReadCloser, error) {
	return dmfs.vfs.Open(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) Create(path string) (io.WriteCloser, error) {
	return dmfs.vfs.Create(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) Remove(path string) error {
	return dmfs.vfs.Remove(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) RemoveAll(path string) error {
	return dmfs.vfs.RemoveAll(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) Rename(oldpath, newpath string) error {
	return dmfs.vfs.Rename(dmfs.convertPath(oldpath), dmfs.convertPath(newpath))
}

func (dmfs *DOSMemoryFileSystem) Mkdir(name string, perm iofs.FileMode) error {
	return dmfs.vfs.Mkdir(dmfs.convertPath(name), perm)
}

func (dmfs *DOSMemoryFileSystem) MkdirAll(path string, perm iofs.FileMode) error {
	return dmfs.vfs.MkdirAll(dmfs.convertPath(path), perm)
}

func (dmfs *DOSMemoryFileSystem) Stat(path string) (iofs.FileInfo, error) {
	return dmfs.vfs.Stat(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) Lstat(path string) (iofs.FileInfo, error) {
	return dmfs.vfs.Lstat(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) ReadDir(path string) ([]iofs.DirEntry, error) {
	return dmfs.vfs.ReadDir(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) Exists(path string) bool {
	return dmfs.vfs.Exists(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) IsDir(path string) bool {
	return dmfs.vfs.IsDir(dmfs.convertPath(path))
}

func (dmfs *DOSMemoryFileSystem) Walk(root string, fn func(path string, info interface{}, err error) error) error {
	return dmfs.vfs.Walk(dmfs.convertPath(root), fn)
}
