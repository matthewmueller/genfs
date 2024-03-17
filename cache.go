package genfs

import (
	"io/fs"

	"github.com/matthewmueller/virt"
)

type Cache interface {
	Get(path string) (*virt.File, error)
	Set(path string, file *virt.File) error
	Link(from string, toPatterns ...string) error
}

// Memory returns a new in-memory cache.
func newMemoryCache() Cache {
	return &memoryCache{
		files: map[string]*virt.File{},
		links: map[string][]string{},
	}
}

type memoryCache struct {
	files map[string]*virt.File
	links map[string][]string
}

func (m *memoryCache) Get(path string) (*virt.File, error) {
	if file, ok := m.files[path]; ok {
		return file, nil
	}
	return nil, fs.ErrNotExist
}

func (m *memoryCache) Set(path string, file *virt.File) error {
	m.files[path] = file
	return nil
}

func (m *memoryCache) Link(from string, toPatterns ...string) error {
	m.links[from] = toPatterns
	return nil
}

type discardCache struct{}

func (d discardCache) Get(path string) (*virt.File, error) {
	return nil, fs.ErrNotExist
}

func (d discardCache) Set(path string, file *virt.File) error {
	return nil
}

func (d discardCache) Link(from string, toPatterns ...string) error {
	return nil
}
