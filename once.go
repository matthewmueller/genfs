package genfs

import (
	"io/fs"
	"sync"
)

// onceBytes ensures we only call a function that returns bytes once
type onceBytes struct {
	o sync.Once
	v []byte
	e error
}

func (d *onceBytes) Do(fn func() ([]byte, error)) ([]byte, error) {
	d.o.Do(func() { d.v, d.e = fn() })
	return d.v, d.e
}

// onceFileInfo ensures we only only call a function that returns fs.FileInfo once
type onceFileInfo struct {
	o sync.Once
	v fs.FileInfo
	e error
}

func (d *onceFileInfo) Do(fn func() (fs.FileInfo, error)) (fs.FileInfo, error) {
	d.o.Do(func() { d.v, d.e = fn() })
	return d.v, d.e
}

// onceDirEntries ensures we only only call a function that returns []fs.DirEntry once
type onceDirEntries struct {
	o sync.Once
	v []fs.DirEntry
	e error
}

func (d *onceDirEntries) Do(fn func() ([]fs.DirEntry, error)) ([]fs.DirEntry, error) {
	d.o.Do(func() { d.v, d.e = fn() })
	return d.v, d.e
}
