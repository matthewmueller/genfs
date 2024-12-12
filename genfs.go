package genfs

import (
	"io/fs"
	"strings"

	"github.com/matthewmueller/genfs/cache"
	"github.com/matthewmueller/genfs/internal/tree"
	"github.com/matthewmueller/virt"
)

type FS fs.FS

type FileGenerator interface {
	GenerateFile(fsys FS, file *File) error
}

type DirGenerator interface {
	GenerateDir(fsys FS, dir *Dir) error
}

type GeneratorFunc func(cache cache.Interface, target string) (*virt.File, error)

func (fn GeneratorFunc) Generate(cache cache.Interface, target string) (*virt.File, error) {
	return fn(cache, target)
}

func New(fsys fs.FS) *FileSystem {
	return &FileSystem{fsys, tree.New(), ".", cache.Discard()}
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
