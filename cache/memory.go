package cache

import (
	"io/fs"

	"github.com/matthewmueller/virt"
)

// Memory returns a new in-memory cache.
func Memory() Interface {
	return &memory{
		files: map[string]*virt.File{},
		links: map[string][]string{},
	}
}

type memory struct {
	files map[string]*virt.File
	links map[string][]string
}

func (m *memory) Get(path string) (*virt.File, error) {
	if file, ok := m.files[path]; ok {
		return file, nil
	}
	return nil, fs.ErrNotExist
}

func (m *memory) Set(path string, file *virt.File) error {
	m.files[path] = file
	return nil
}

func (m *memory) Link(from string, toPatterns ...string) error {
	m.links[from] = toPatterns
	return nil
}
