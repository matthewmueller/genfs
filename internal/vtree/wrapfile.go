package vtree

import (
	"io"
	"io/fs"

	"github.com/matthewmueller/virt"
)

// wrapFile turns a virt.File into an fs.File. Unlike virt.Open
func wrapFile(fsys fs.FS, path string, file fs.File) fs.File {
	return &fsFile{file, fsys, path, 0}
}

type fsFile struct {
	fs.File
	fsys   fs.FS
	path   string
	offset int64
}

var _ fs.File = (*fsFile)(nil)
var _ io.Seeker = (*fsFile)(nil)
var _ fs.ReadDirFile = (*fsFile)(nil)

func (f *fsFile) ReadDir(count int) ([]fs.DirEntry, error) {
	des, err := fs.ReadDir(f.fsys, f.path)
	if err != nil {
		return nil, err
	}
	offset := int(f.offset)
	n := len(des) - offset
	if count > 0 && n > count {
		n = count
	}
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	entries := make([]fs.DirEntry, n)
	for i := range entries {
		entries[i] = des[offset+i]
	}
	f.offset += int64(n)
	return entries, nil
}

func (f *fsFile) Seek(offset int64, whence int) (int64, error) {
	seeker, ok := f.File.(io.Seeker)
	if !ok {
		return 0, &fs.PathError{
			Op:   "seek",
			Path: f.path,
			Err:  fs.ErrInvalid,
		}
	}
	return seeker.Seek(offset, whence)
}

func wrapEntry(tree fs.FS, de *virt.DirEntry) fs.DirEntry {
	return &dirEntry{de, tree}
}

type dirEntry struct {
	*virt.DirEntry
	tree fs.FS
}

func (de *dirEntry) Info() (fs.FileInfo, error) {
	return fs.Stat(de.tree, de.Path)
}
