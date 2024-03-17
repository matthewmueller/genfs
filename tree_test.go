package genfs

import (
	"io/fs"
	"testing"

	"github.com/matryer/is"
)

type treeGenerator struct{ label string }

func (g *treeGenerator) Generate(cache Cache, target string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func (g *treeGenerator) String() string {
	return g.label
}

var ag = &treeGenerator{"a"}
var bg = &treeGenerator{"b"}
var cg = &treeGenerator{"c"}
var eg = &treeGenerator{"e"}
var fg = &treeGenerator{"f"}

func TestTreeInsert(t *testing.T) {
	is := is.New(t)
	n := newTree()
	n.Insert("a", modeGen, ag)
	n.Insert("b", modeGenDir, bg)
	n.Insert("b/c", modeGenDir, cg)
	n.Insert("b/c/e", modeGen, eg)
	n.Insert("b/c/f", modeGen, fg)
	expect := `. mode=d-
├── a mode=-g generator=a
└── b mode=dg generator=b
    └── c mode=dg generator=c
        ├── e mode=-g generator=e
        └── f mode=-g generator=f
`
	is.Equal(n.Print(), expect)
}

func TestTreeFiller(t *testing.T) {
	is := is.New(t)
	n := newTree()
	n.Insert("a", modeGen, ag)
	n.Insert("b/c/e", modeGen, eg)
	n.Insert("b/c/f", modeGen, fg)
	n.Insert("b/c", modeGenDir, cg)
	expect := `. mode=d-
├── a mode=-g generator=a
└── b mode=d-
    └── c mode=dg generator=c
        ├── e mode=-g generator=e
        └── f mode=-g generator=f
`
	is.Equal(n.Print(), expect)
}

func TestTreeFindPrefix(t *testing.T) {
	s := is.New(t)
	n := newTree()
	n.Insert("a", modeGen, ag)
	n.Insert("b", modeGenDir, bg)
	n.Insert("b/c", modeGenDir, cg)
	n.Insert("b/c/e", modeGen, eg)
	n.Insert("b/c/f", modeGen, fg)
	f, ok := n.FindPrefix("a")
	s.True(ok)
	s.Equal(f.Path, "a")
	// File generators must be an exact match
	f, ok = n.FindPrefix("a/d")
	s.Equal(ok, false)
	s.Equal(f, nil)
	f, ok = n.FindPrefix("b/c/h")
	s.True(ok)
	s.Equal(f.Path, "b/c")
	f, ok = n.FindPrefix("c")
	s.Equal(ok, true)
	s.Equal(f.Path, ".")
	f, ok = n.FindPrefix("c")
	s.Equal(ok, true)
	s.Equal(f.Path, ".")
	// Special case
	f, ok = n.FindPrefix(".")
	s.True(ok)
	s.Equal(f.Path, ".")
}

func TestTreeDelete(t *testing.T) {
	is := is.New(t)
	n := newTree()
	n.Insert("a", modeGen, ag)
	n.Insert("b", modeGenDir, bg)
	n.Insert("b/c", modeGenDir, cg)
	n.Insert("b/c/e", modeGen, eg)
	n.Insert("b/c/f", modeGen, fg)
	n.Delete("b/c")
	expect := `. mode=d-
├── a mode=-g generator=a
└── b mode=dg generator=b
`
	is.Equal(n.Print(), expect)
}

func TestTreeFillerDirNowGeneratorFile(t *testing.T) {
	is := is.New(t)
	n := newTree()
	n.Insert("bud/node_modules", modeGenDir, ag)
	n.Insert("bud/node_modules/runtime/hot", modeGen, bg)
	node, ok := n.FindPrefix("bud/node_modules/runtime/svelte")
	is.True(ok)
	is.Equal(node.Path, "bud/node_modules/runtime")
	// Check that parent is a directory
	parent, ok := n.Find("bud/node_modules/runtime")
	is.True(ok)
	is.True(parent.Mode.IsDir())
	n.Insert("bud/node_modules/runtime", modeGen, cg)
	// Check that parent is a file
	parent, ok = n.Find("bud/node_modules/runtime")
	is.True(ok)
	is.Equal(parent.Mode, modeGen)
}

func TestTreeGeneratorAndDirectory(t *testing.T) {
	is := is.New(t)
	n := newTree()
	n.Insert("bud/node_modules", modeGenDir, ag)
	n.Insert("bud/node_modules/runtime", modeGen, bg)
	n.Insert("bud/node_modules/runtime/hot", modeGen, cg)
	node, ok := n.FindPrefix("bud/node_modules/runtime")
	is.True(ok)
	is.Equal(node.Path, "bud/node_modules/runtime")
	node, ok = n.FindPrefix("bud/node_modules/runtime/hot")
	is.True(ok)
	is.Equal(node.Path, "bud/node_modules/runtime/hot")
}
