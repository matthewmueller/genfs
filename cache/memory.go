package cache

import (
	"io/fs"

	"github.com/matthewmueller/virt"
)

// Memory returns a new in-memory cache.
func Memory() *Mem {
	return &Mem{
		files: map[string]*virt.File{},
		links: map[string][]string{},
	}
}

// Mem is an in-memory cache.
type Mem struct {
	files map[string]*virt.File
	links map[string][]string
}

func (m *Mem) Get(path string) (*virt.File, error) {
	if file, ok := m.files[path]; ok {
		return file, nil
	}
	return nil, fs.ErrNotExist
}

func (m *Mem) Set(path string, file *virt.File) error {
	m.files[path] = file
	return nil
}

func (m *Mem) Link(from string, toPatterns ...string) error {
	m.links[from] = toPatterns
	return nil
}

func (m *Mem) Clear() {
	m.files = map[string]*virt.File{}
	m.links = map[string][]string{}
}
