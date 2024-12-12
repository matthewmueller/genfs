package cache

import (
	"github.com/matthewmueller/virt"
)

type Interface interface {
	Get(path string) (*virt.File, error)
	Set(path string, file *virt.File) error
	Link(from string, to ...string) error
}
