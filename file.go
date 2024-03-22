package genfs

import (
	"bytes"
	"io/fs"
)

type File struct {
	target string
	path   string
	mode   fs.FileMode
	data   *bytes.Buffer
}

func (f *File) Target() string {
	return f.target
}

func (f *File) Path() string {
	return f.path
}

func (f *File) Relative() string {
	return relativePath(f.path, f.target)
}

func (f *File) Mode() fs.FileMode {
	return f.mode
}

func (f *File) Write(p []byte) (n int, err error) {
	return f.data.Write(p)
}

func (f *File) WriteString(s string) (n int, err error) {
	return f.data.WriteString(s)
}

func (f *File) Read(p []byte) (n int, err error) {
	return f.data.Read(p)
}
