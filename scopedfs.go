package genfs

import (
	"io/fs"

	"github.com/matthewmueller/genfs/cache"
)

type scopedFS struct {
	fsys  fs.FS
	cache cache.Interface
	from  string
}

func (s scopedFS) Open(name string) (fs.File, error) {
	return s.fsys.Open(name)
}
