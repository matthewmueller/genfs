package genfs

import (
	"bytes"
	"io/fs"
	"path"

	"github.com/matthewmueller/genfs/internal/cache"
	"github.com/matthewmueller/genfs/internal/tree"
	"github.com/matthewmueller/virt"
)

type Dir struct {
	fsys   fs.FS
	tree   *tree.Tree
	target string
	dir    string
	mode   fs.FileMode
}

func (d *Dir) Target() string {
	return d.target
}

func (d *Dir) Path() string {
	return d.dir
}

func (d *Dir) Mode() fs.FileMode {
	return d.mode
}

func (d *Dir) Relative() string {
	return relativePath(d.dir, d.target)
}

func (d *Dir) GenerateFile(relpath string, fn func(fsys FS, file *File) error) error {
	return d.tree.GenerateFile(path.Join(d.dir, relpath), GeneratorFunc(func(cache cache.Interface, target string) (*virt.File, error) {
		file := &File{target, relpath, fs.FileMode(0), &bytes.Buffer{}}
		fsys := scopedFS{d.fsys, cache, relpath}
		if err := fn(fsys, file); err != nil {
			return nil, err
		}
		return &virt.File{
			Path: relpath,
			Mode: file.Mode(),
			Data: file.data.Bytes(),
		}, nil
	}))
}

func (d *Dir) FileGenerator(relpath string, generator FileGenerator) error {
	return d.GenerateFile(relpath, generator.GenerateFile)
}

func (d *Dir) GenerateDir(reldir string, fn func(fsys FS, dir *Dir) error) error {
	reldir = path.Join(d.dir, reldir)
	return d.tree.GenerateDir(reldir, GeneratorFunc(func(cache cache.Interface, target string) (*virt.File, error) {
		dir := &Dir{d.fsys, d.tree, target, reldir, fs.ModeDir}
		fsys := scopedFS{d.fsys, cache, reldir}
		if err := fn(fsys, dir); err != nil {
			return nil, err
		}
		return &virt.File{
			Path: reldir,
			Mode: dir.mode,
			// Intentionally nil, filled in by the tree
			Entries: nil,
		}, nil
	}))
}

func (d *Dir) DirGenerator(reldir string, generator DirGenerator) error {
	return d.GenerateDir(reldir, generator.GenerateDir)
}
