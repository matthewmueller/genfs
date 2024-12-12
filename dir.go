package genfs

import (
	"bytes"
	"io/fs"
	"path"

	"github.com/matthewmueller/genfs/cache"
	"github.com/matthewmueller/genfs/internal/tree"
	"github.com/matthewmueller/virt"
)

type Dir struct {
	fsys   *FileSystem
	tree   *tree.Tree
	target string
	dir    string
	mode   fs.FileMode
	root   string
}

func (d *Dir) Target() string {
	return path.Join(d.root, d.target)
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
		if cached, err := cache.Get(target); nil == err {
			return cached, nil
		}
		file := &File{target, relpath, fs.FileMode(0), &bytes.Buffer{}, d.root}
		fsys := scopedFS{d.fsys, cache, relpath}
		if err := fn(fsys, file); err != nil {
			return nil, err
		}
		vfile := &virt.File{
			Path: relpath,
			Mode: file.Mode(),
			Data: file.data.Bytes(),
		}
		if err := cache.Set(target, vfile); err != nil {
			return nil, err
		}
		return vfile, nil
	}))
}

func (d *Dir) FileGenerator(relpath string, generator FileGenerator) error {
	return d.GenerateFile(relpath, generator.GenerateFile)
}

func (d *Dir) GenerateDir(reldir string, fn func(fsys FS, dir *Dir) error) error {
	reldir = path.Join(d.dir, reldir)
	return d.tree.GenerateDir(reldir, GeneratorFunc(func(cache cache.Interface, target string) (*virt.File, error) {
		if cached, err := cache.Get(reldir); nil == err {
			return cached, nil
		}
		dir := &Dir{d.fsys, d.tree, target, reldir, fs.ModeDir, d.root}
		fsys := scopedFS{d.fsys, cache, reldir}
		if err := fn(fsys, dir); err != nil {
			return nil, err
		}
		vdir := &virt.File{
			Path: reldir,
			Mode: dir.mode,
			// Intentionally nil, filled in by the tree
			Entries: nil,
		}
		if err := cache.Set(reldir, vdir); err != nil {
			return nil, err
		}
		return vdir, nil
	}))
}

func (d *Dir) DirGenerator(reldir string, generator DirGenerator) error {
	return d.GenerateDir(reldir, generator.GenerateDir)
}
