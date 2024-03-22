package cache

import (
	"io/fs"

	"github.com/matthewmueller/virt"
)

func Discard() Interface {
	return discardCache{}
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
