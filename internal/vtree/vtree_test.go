package vtree_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/matryer/is"
	"github.com/matthewmueller/genfs/internal/vtree"
	"github.com/matthewmueller/virt"
)

func TestError(t *testing.T) {
	is := is.New(t)
	dir := t.TempDir()
	err := os.WriteFile(dir, []byte("hello"), 0644)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), `is a directory`))
}

func TestNew(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.Equal(tree.Print(), `. mode=d- generators=0
`)
}

type treeGenerator struct{ label string }

func (g *treeGenerator) Generate(cache vtree.Cache, target string) (*virt.File, error) {
	return nil, fs.ErrNotExist
}

var ag = &treeGenerator{"a"}
var bg = &treeGenerator{"b"}
var cg = &treeGenerator{"c"}
var eg = &treeGenerator{"e"}
var fg = &treeGenerator{"f"}

func TestSimpleGenerateFile(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("a/b/c", ag))
	expect := `. mode=d- generators=0
└── a mode=d- generators=0
    └── b mode=d- generators=0
        └── c mode=-g generators=1
`
	is.Equal(tree.Print(), expect)
}

func TestRootDir(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateDir(".", ag))
	expect := `. mode=dg generators=1
`
	is.Equal(tree.Print(), expect)
}

func TestSimpleGenerateDir(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateDir("a", ag))
	expect := `. mode=d- generators=0
└── a mode=dg generators=1
`
	is.Equal(tree.Print(), expect)
}

func TestSampleGen(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("./a", ag))
	is.NoErr(tree.GenerateDir("b", bg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	expect := `. mode=d- generators=0
├── a mode=-g generators=1
└── b mode=dg generators=1
    └── c mode=dg generators=1
        ├── e mode=-g generators=1
        └── f mode=-g generators=1
`
	is.Equal(tree.Print(), expect)
}

func TestTreeFiller(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("a", ag))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	expect := `. mode=d- generators=0
├── a mode=-g generators=1
└── b mode=d- generators=0
    └── c mode=dg generators=1
        ├── e mode=-g generators=1
        └── f mode=-g generators=1
`
	is.Equal(tree.Print(), expect)
}

func TestTreeFindPrefix(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("a", ag))
	is.NoErr(tree.GenerateDir("b", bg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	m, ok := tree.FindPrefix("a")
	is.True(ok)
	is.Equal(m.Path, "a")
	// File generators must be an exact match
	m, ok = tree.FindPrefix("a/d")
	is.Equal(ok, false)
	is.Equal(m, nil)
	m, ok = tree.FindPrefix("b/c/h")
	is.True(ok)
	is.Equal(m.Path, "b/c")
	m, ok = tree.FindPrefix("c")
	is.Equal(ok, true)
	is.Equal(m.Path, ".")
	m, ok = tree.FindPrefix("c")
	is.Equal(ok, true)
	is.Equal(m.Path, ".")
	// Special case
	m, ok = tree.FindPrefix(".")
	is.True(ok)
	is.Equal(m.Path, ".")
}

func TestTreeDelete(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("a", ag))
	is.NoErr(tree.GenerateDir("b", bg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	tree.Delete("b/c")
	expect := `. mode=d- generators=0
├── a mode=-g generators=1
└── b mode=dg generators=1
`
	is.Equal(tree.Print(), expect)
}

func TestTreeDirToGenFile(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateDir("bud/node_modules", ag))
	is.NoErr(tree.GenerateFile("bud/node_modules/runtime/hot", bg))
	match, ok := tree.FindPrefix("bud/node_modules/runtime/svelte")
	is.True(ok)
	is.Equal(match.Path, "bud/node_modules/runtime")
	// Check that parent is a directory
	match, ok = tree.Find("bud/node_modules/runtime")
	is.True(ok)
	is.True(match.Mode.IsDir())
	// Dir can't become a generator file
	err := tree.GenerateFile("bud/node_modules/runtime", cg)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "already a directory"))
}

func TestTreeGenFileToGenDirectory(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	tree.GenerateDir("bud/node_modules", ag)
	tree.GenerateFile("bud/node_modules/runtime", bg)
	err := tree.GenerateFile("bud/node_modules/runtime/hot", cg)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "path is already a file"))
}

func TestTreeRootDirGenerator(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	tree.GenerateDir(".", ag)
	match, ok := tree.FindPrefix("index.html")
	is.True(ok)
	is.Equal(match.Path, ".")
	is.True(match.Mode.IsGen())
	expect := `. mode=dg generators=1
`
	is.Equal(tree.Print(), expect)
}

type Func func(cache vtree.Cache, target string) (*virt.File, error)

func (f Func) Generate(cache vtree.Cache, target string) (*virt.File, error) {
	return f(cache, target)
}

func TestTreeSharedDirGenerator(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateDir(".", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{
			Path: ".",
			Mode: fs.ModeDir,
			Entries: []fs.DirEntry{
				&virt.File{Path: "index.html", Data: []byte("<h1>index</h1>")},
				&virt.File{Path: "about.html", Data: []byte("<h1>about</h1>")},
			},
		}, nil
	})))
	is.NoErr(tree.GenerateDir(".", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{
			Path: ".",
			Mode: fs.ModeDir,
			Entries: []fs.DirEntry{
				&virt.File{Path: "favicon.ico", Data: []byte("favicon.ico")},
			},
		}, nil
	})))
	match, ok := tree.FindPrefix("index.html")
	is.True(ok)
	is.Equal(match.Path, ".")
	is.True(match.Mode.IsGenDir())
	expect := `. mode=dg generators=2
`
	is.Equal(tree.Print(), expect)
	vfile, err := match.Generate(nil, ".")
	is.NoErr(err)
	is.Equal(vfile.Path, ".")
	is.True(vfile.Mode.IsDir())
	is.Equal(len(vfile.Entries), 3)
	is.Equal(vfile.Entries[0].Name(), "about.html")
	is.True(!vfile.Entries[0].IsDir())
	is.Equal(vfile.Entries[1].Name(), "favicon.ico")
	is.True(!vfile.Entries[1].IsDir())
	is.Equal(vfile.Entries[2].Name(), "index.html")
	is.True(!vfile.Entries[2].IsDir())
}

func TestTreeSharedDirAndDirGenerator(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("favicon.ico", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{}, nil
	})))
	tree.GenerateDir(".", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{}, nil
	}))
	tree.GenerateDir(".", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{}, nil
	}))
	match, ok := tree.FindPrefix("index.html")
	is.True(ok)
	is.Equal(match.Path, ".")
	is.True(match.Mode.IsGenDir())
	expect := `. mode=dg generators=2
└── favicon.ico mode=-g generators=1
`
	is.Equal(tree.Print(), expect)

}

func TestTreeGenDirFileGenOverride(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateDir("a", ag))
	is.NoErr(tree.GenerateDir("a", bg))
	err := tree.GenerateFile("a", cg)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "already a directory"))
	match, ok := tree.Find("a")
	is.True(ok)
	is.Equal(match.Path, "a")
	is.True(match.Mode.IsGen())
	expect := `. mode=d- generators=0
└── a mode=dg generators=2
`
	is.Equal(tree.Print(), expect)
}

func TestTreeFileGenOverride(t *testing.T) {
	is := is.New(t)
	tree := vtree.New()
	is.NoErr(tree.GenerateFile("a.txt", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{
			Path: target,
			Data: []byte("1"),
		}, nil
	})))
	is.NoErr(tree.GenerateFile("a.txt", Func(func(cache vtree.Cache, target string) (*virt.File, error) {
		return &virt.File{
			Path: target,
			Data: []byte("2"),
		}, nil
	})))
	match, ok := tree.Find("a.txt")
	is.True(ok)
	is.Equal(match.Path, "a.txt")
	is.True(match.Mode.IsGen())
	expect := `. mode=d- generators=0
└── a.txt mode=-g generators=1
`
	is.Equal(tree.Print(), expect)
	vfile, err := match.Generate(nil, "a.txt")
	is.NoErr(err)
	is.Equal(vfile.Path, "a.txt")
	is.True(vfile.Mode.IsRegular())
	is.Equal(string(vfile.Data), "2")
}
