package genfs

import (
	"bytes"
	"io/fs"

	"github.com/matthewmueller/virt"
)

type FileServer interface {
	ServeFile(fsys FS, file *File) error
}

type ServeFile func(fsys FS, file *File) error

func (fn ServeFile) ServeFile(fsys FS, file *File) error {
	return fn(fsys, file)
}

type fileServer struct {
	genfs fs.FS
	path  string
	fn    func(fsys FS, file *File) error
}

var _ generator = (*fileServer)(nil)

func (f *fileServer) Generate(cache Cache, target string) (fs.File, error) {
	if file, err := cache.Get(target); err == nil {
		return virt.Open(file), nil
	}
	// Always return an empty directory if we request the root
	if f.path == target {
		return virt.Open(&virt.File{
			Path: f.path,
			Mode: fs.ModeDir,
		}), nil
	}
	scopedFS := &scopedFS{cache, f.genfs, target}
	file := &File{new(bytes.Buffer), f.path, target}
	if err := f.fn(scopedFS, file); err != nil {
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
	return virt.Open(vfile), nil
}
