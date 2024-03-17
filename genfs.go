package genfs

import (
	"io/fs"
	"strings"

	"github.com/matthewmueller/virt"
)

type Generators interface {
	GenerateFile(path string, fn func(fsys FS, file *File) error)
	FileGenerator(path string, generator FileGenerator)
	GenerateDir(path string, fn func(fsys FS, dir *Dir) error)
	DirGenerator(path string, generator DirGenerator)
	ServeFile(dir string, fn func(fsys FS, file *File) error)
	FileServer(dir string, server FileServer)
	GenerateExternal(path string, fn func(fsys FS, file *External) error)
	ExternalGenerator(path string, generator ExternalGenerator)
}

type FS interface {
	fs.FS
	fs.ReadDirFS
	fs.GlobFS
	Watch(patterns ...string) error
}

type generator interface {
	Generate(cache Cache, target string) (fs.File, error)
}

func New() *FileSystem {
	tree := newTree()
	session := &Session{discardCache{}, &virt.List{}, tree}
	return &FileSystem{tree, session}
}

type FileSystem struct {
	tree    *tree    // Tree for the generators and filler nodes
	session *Session // Default session
}

func (f *FileSystem) GenerateFile(path string, fn func(fsys FS, file *File) error) {
	fileg := &fileGenerator{f, path, fn}
	f.tree.Insert(path, modeGen, fileg)
}

func (f *FileSystem) FileGenerator(path string, generator FileGenerator) {
	f.GenerateFile(path, generator.GenerateFile)
}

func (f *FileSystem) GenerateDir(path string, fn func(fsys FS, dir *Dir) error) {
	dirg := &dirGenerator{f, f.tree, path, fn}
	f.tree.Insert(path, modeGenDir, dirg)
}

func (f *FileSystem) DirGenerator(path string, generator DirGenerator) {
	f.GenerateDir(path, generator.GenerateDir)
}

func (f *FileSystem) ServeFile(dir string, fn func(fsys FS, file *File) error) {
	server := &fileServer{f, dir, fn}
	f.tree.Insert(dir, modeGenDir, server)
}

func (f *FileSystem) FileServer(dir string, server FileServer) {
	f.ServeFile(dir, server.ServeFile)
}

func (f *FileSystem) GenerateExternal(path string, fn func(fsys FS, file *External) error) {
	fileg := &externalGenerator{f, path, fn}
	f.tree.Insert(path, modeGen, fileg)
}

func (f *FileSystem) ExternalGenerator(path string, generator ExternalGenerator) {
	f.GenerateExternal(path, generator.GenerateExternal)
}

func (f *FileSystem) Session() *Session {
	return &Session{
		Cache: newMemoryCache(),
		FS:    f.session.FS,
		tree:  f.tree,
	}
}

func (f *FileSystem) Open(target string) (fs.File, error) {
	return f.session.Open(target)
}

func (f *FileSystem) openFrom(previous string, target string) (fs.File, error) {
	return f.session.openFrom(previous, target)
}

func (f *FileSystem) ReadDir(target string) ([]fs.DirEntry, error) {
	return f.session.ReadDir(target)
}

func relativePath(base, target string) string {
	rel := strings.TrimPrefix(target, base)
	if rel == "" {
		return "."
	} else if rel[0] == '/' {
		rel = rel[1:]
	}
	return rel
}
