package genfs

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/matthewmueller/glob"
)

type scopedFS struct {
	cache Cache
	genfs fs.FS
	from  string // generator path
}

var _ FS = (*scopedFS)(nil)

// Open implements fs.FS
func (f *scopedFS) Open(name string) (fs.File, error) {
	f.cache.Link(f.from, name)
	file, err := f.genfs.Open(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// Watch the paths for changes
func (f *scopedFS) Watch(patterns ...string) error {
	return f.cache.Link(f.from, patterns...)
}

// ReadDir implements fs.ReadDirFS
func (f *scopedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	des, err := fs.ReadDir(f.genfs, name)
	if err != nil {
		return nil, err
	}
	// Link the directory to react to future changes
	toPattern := fmt.Sprintf(`{%s,%s}`, name, path.Dir(name))
	if err := f.cache.Link(f.from, toPattern); err != nil {
		return nil, err
	}
	return des, nil
}

// Glob implements fs.GlobFS
func (f *scopedFS) Glob(pattern string) ([]string, error) {
	matches, err := glob.MatchFS(f.genfs, pattern)
	if err != nil {
		return nil, err
	}
	// Link the pattern to react to future changes
	if err := f.cache.Link(f.from, pattern); err != nil {
		return nil, err
	}
	return matches, nil
}
