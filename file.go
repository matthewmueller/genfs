package genfs

import (
	"bytes"
	"io"
	"io/fs"
	"path"

	"github.com/matthewmueller/genfs/internal/vtree"
	"github.com/matthewmueller/virt"
)

type File struct {
	data *bytes.Buffer

	// Target and path are the same when called within GenerateFile, but not
	// always the same when called within ServeFile
	path   string
	target string
}

var _ io.Writer = (*File)(nil)
var _ io.Reader = (*File)(nil)

func (f *File) Write(b []byte) (int, error) {
	return f.data.Write(b)
}

func (f *File) WriteString(s string) (int, error) {
	return f.data.WriteString(s)
}

func (f *File) Read(b []byte) (int, error) {
	return f.data.Read(b)
}

func (f *File) ReadString(delim byte) (string, error) {
	return f.data.ReadString(delim)
}

func (f *File) Target() string {
	return f.target
}

func (f *File) Relative() string {
	return relativePath(f.path, f.target)
}

func (f *File) Path() string {
	return f.path
}

// Ext returns the extension to the target file path (e.g. `.svelte`)
func (f *File) Ext() string {
	return path.Ext(f.target)
}

func (f *File) Mode() fs.FileMode {
	return fs.FileMode(0)
}

type FileGenerator interface {
	GenerateFile(fsys FS, file *File) error
}

type GenerateFile func(fsys FS, file *File) error

func (fn GenerateFile) GenerateFile(fsys FS, file *File) error {
	return fn(fsys, file)
}

type fileGenerator struct {
	genfs fs.FS
	path  string
	fn    func(fsys FS, file *File) error
}

func (f *fileGenerator) Generate(cache vtree.Cache, target string) (*virt.File, error) {
	if target != f.path {
		return nil, formatError(fs.ErrNotExist, "%q path doesn't match %q target", f.path, target)
	}
	if vfile, err := cache.Get(target); err == nil {
		return vfile, nil
	}
	file := &File{new(bytes.Buffer), f.path, target}
	scoped := &scopedFS{cache, f.genfs, target}
	if err := f.fn(scoped, file); err != nil {
		return nil, err
	}
	vfile := &virt.File{
		Path: target,
		Mode: fs.FileMode(0),
		Data: file.data.Bytes(),
	}
	if err := cache.Set(target, vfile); err != nil {
		return nil, err
	}
	return vfile, nil
}
