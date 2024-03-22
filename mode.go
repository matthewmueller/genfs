package genfs

import "io/fs"

type Mode uint8

const (
	ModeDir Mode = 1 << iota
	ModeGen
	ModeGenDir = ModeGen | ModeDir
)

func (m Mode) IsDir() bool {
	return m&ModeDir != 0
}

func (m Mode) IsGenFile() bool {
	return m == ModeGen
}

func (m Mode) IsGen() bool {
	return m&ModeGen != 0
}

func (m Mode) IsGenDir() bool {
	return m == ModeGenDir
}

func (m Mode) FileMode() fs.FileMode {
	mode := fs.FileMode(0)
	if m.IsDir() {
		mode |= fs.ModeDir
	}
	return mode
}

func (m Mode) String() string {
	var s string
	if m.IsDir() {
		s += "d"
	} else {
		s += "-"
	}
	if m.IsGen() {
		s += "g"
	} else {
		s += "-"
	}
	return s
}
