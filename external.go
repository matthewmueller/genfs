package genfs

import (
	"io/fs"

	"github.com/matthewmueller/virt"
)

type External struct {
	target string
}

func (e *External) Path() string {
	return e.target
}

func (e *External) Target() string {
	return e.target
}

func (e *External) Mode() fs.FileMode {
	return fs.FileMode(0)
}

type ExternalGenerator interface {
	GenerateExternal(fsys FS, file *External) error
}

type externalGenerator struct {
	genfs fs.FS
	path  string
	fn    func(fsys FS, e *External) error
}

func (e *externalGenerator) Generate(cache Cache, target string) (*virt.File, error) {
	if target != e.path {
		return nil, formatError(fs.ErrNotExist, "%q path doesn't match %q target", e.path, target)
	}
	if _, err := cache.Get(target); err == nil {
		return nil, fs.ErrNotExist
	}
	scoped := &scopedFS{cache, e.genfs, target}
	file := &External{target}
	if err := e.fn(scoped, file); err != nil {
		return nil, err
	}
	vfile := &virt.File{
		Path: target,
		Mode: file.Mode(),
	}
	if err := cache.Set(target, vfile); err != nil {
		return nil, err
	}
	return nil, fs.ErrNotExist
}
