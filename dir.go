package genfs

import (
	"io/fs"
	gopath "path"

	"github.com/matthewmueller/genfs/internal/vtree"
	"github.com/matthewmueller/virt"
)

type Dir struct {
	cache  Cache
	genfs  *FileSystem
	path   string // Current directory path
	target string // Final target path
	tree   *vtree.Tree
}

var _ Interface = (*Dir)(nil)

func (d *Dir) Target() string {
	return d.target
}

func (d *Dir) Relative() string {
	return relativePath(d.path, d.target)
}

func (d *Dir) Path() string {
	return d.path
}

func (d *Dir) Mode() fs.FileMode {
	return fs.ModeDir
}

func (d *Dir) GenerateFile(path string, fn func(fsys FS, file *File) error) {
	fpath := gopath.Join(d.path, path)
	fileg := &fileGenerator{d.genfs, fpath, fn}
	d.tree.GenerateFile(fpath, fileg)
}

func (d *Dir) FileGenerator(path string, generator FileGenerator) {
	d.GenerateFile(path, generator.GenerateFile)
}

func (d *Dir) GenerateDir(path string, fn func(fsys FS, dir *Dir) error) {
	fpath := gopath.Join(d.path, path)
	dirg := &dirGenerator{d.genfs, d.tree, fpath, fn}
	d.tree.GenerateDir(fpath, dirg)
}

func (d *Dir) DirGenerator(path string, generator DirGenerator) {
	d.GenerateDir(path, generator.GenerateDir)
}

func (d *Dir) ServeFile(path string, fn func(fsys FS, file *File) error) {
	fpath := gopath.Join(d.path, path)
	server := &fileServer{d.genfs, fpath, fn}
	d.tree.GenerateDir(fpath, server)
}

func (d *Dir) FileServer(path string, server FileServer) {
	d.ServeFile(path, server.ServeFile)
}

func (d *Dir) GenerateExternal(path string, fn func(fsys FS, file *External) error) {
	fpath := gopath.Join(d.path, path)
	fileg := &externalGenerator{d.genfs, fpath, fn}
	d.tree.GenerateFile(fpath, fileg)
}
func (d *Dir) ExternalGenerator(path string, generator ExternalGenerator) {
	d.GenerateExternal(path, generator.GenerateExternal)
}

// type mountGenerator struct {
// 	dir   string
// 	mount fs.FS
// }

// func (g *mountGenerator) Generate(cache Cache, target string) (*virt.File, error) {
// 	file, err := g.mount.Open(relativePath(g.dir, target))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return virt.From(file)
// }

// func (d *Dir) Mount(mount fs.FS) error {
// 	err := fs.WalkDir(mount, ".", func(path string, de fs.DirEntry, err error) error {
// 		if err != nil {
// 			return err
// 		}
// 		// Don't overwrite the existing root directory
// 		if path == "." {
// 			return nil
// 		}
// 		fpath := gopath.Join(d.path, path)
// 		mode := modeGen
// 		if de.IsDir() {
// 			mode = modeGenDir
// 		}
// 		d.tree.Insert(fpath, mode, &mountGenerator{d.path, mount})
// 		return nil
// 	})
// 	if err != nil {
// 		return fmt.Errorf("budfs: mount error. %w", err)
// 	}
// 	return nil
// }

type DirGenerator interface {
	GenerateDir(fsys FS, dir *Dir) error
}

type GenerateDir func(fsys FS, dir *Dir) error

func (fn GenerateDir) GenerateDir(fsys FS, dir *Dir) error {
	return fn(fsys, dir)
}

type dirGenerator struct {
	genfs *FileSystem
	tree  *vtree.Tree
	path  string
	fn    func(fsys FS, dir *Dir) error
}

func (d *dirGenerator) Generate(cache vtree.Cache, target string) (*virt.File, error) {
	if vfile, err := cache.Get(d.path); err == nil {
		return vfile, nil
	}
	// Run the directory generator function
	scopedFS := &scopedFS{cache, d.genfs, d.path}
	dir := &Dir{cache, d.genfs, d.path, target, d.tree}
	if err := d.fn(scopedFS, dir); err != nil {
		return nil, err
	}
	// Traverse into the directory looking for the target
	if d.path != target {
		return d.genfs.openFrom(d.path, target)
		// if err != nil {
		// 	return nil, err
		// }
		// fmt.Println(d.path, vfile.Path, string(vfile.Data))
		// return vfile, nil
	}
	vfile := &virt.File{
		Path:    d.path,
		Mode:    fs.ModeDir,
		Entries: nil, // Entries get filled in on-demand.
	}
	// Cache the directory entry
	if err := cache.Set(d.path, vfile); err != nil {
		return nil, err
	}
	// Return the virtual directory
	return vfile, nil
}
