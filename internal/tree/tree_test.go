package tree_test

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/matryer/is"
	"github.com/matthewmueller/genfs/internal/cache"
	"github.com/matthewmueller/genfs/internal/tree"
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
	tree := tree.New()
	is.Equal(tree.Print(), `. mode=d-
`)
}

type treeGenerator struct{ label string }

func (g *treeGenerator) Generate(_ cache.Interface, target string) (*virt.File, error) {
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

func TestSimpleGenerateFile(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateFile("a/b/c", cg))
	expect := `. mode=d-
└── a mode=d-
    └── b mode=d-
        └── c mode=-g generators=c
`
	is.Equal(tree.Print(), expect)
}

func TestRootDir(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateDir(".", ag))
	expect := `. mode=dg generators=a
`
	is.Equal(tree.Print(), expect)
}

func TestSimpleGenerateDir(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateDir("a", ag))
	expect := `. mode=d-
└── a mode=dg generators=a
`
	is.Equal(tree.Print(), expect)
}

func TestSampleGen(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateFile("./a", ag))
	is.NoErr(tree.GenerateDir("b", bg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	expect := `. mode=d-
├── a mode=-g generators=a
└── b mode=dg generators=b
    └── c mode=dg generators=c
        ├── e mode=-g generators=e
        └── f mode=-g generators=f
`
	is.Equal(tree.Print(), expect)
}

func TestTreeFiller(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateFile("a", ag))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	expect := `. mode=d-
├── a mode=-g generators=a
└── b mode=d-
    └── c mode=dg generators=c
        ├── e mode=-g generators=e
        └── f mode=-g generators=f
`
	is.Equal(tree.Print(), expect)
}

func TestTreeFindPrefix(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
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
	tree := tree.New()
	is.NoErr(tree.GenerateFile("a", ag))
	is.NoErr(tree.GenerateDir("b", bg))
	is.NoErr(tree.GenerateDir("b/c", cg))
	is.NoErr(tree.GenerateFile("b/c/e", eg))
	is.NoErr(tree.GenerateFile("b/c/f", fg))
	tree.Delete("b/c")
	expect := `. mode=d-
├── a mode=-g generators=a
└── b mode=dg generators=b
`
	is.Equal(tree.Print(), expect)
}

func TestTreeDirToGenFile(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
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
	tree := tree.New()
	tree.GenerateDir("bud/node_modules", ag)
	tree.GenerateFile("bud/node_modules/runtime", bg)
	err := tree.GenerateFile("bud/node_modules/runtime/hot", cg)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "path is already a file"))
}

func TestTreeRootDirGenerator(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	tree.GenerateDir(".", ag)
	match, ok := tree.FindPrefix("index.html")
	is.True(ok)
	is.Equal(match.Path, ".")
	is.True(match.Mode.IsGen())
	expect := `. mode=dg generators=a
`
	is.Equal(tree.Print(), expect)
}

func Func(label string, fn func(_ cache.Interface, target string) (*virt.File, error)) tree.Generator {
	return &funcGenerator{label, fn}
}

type funcGenerator struct {
	label string
	fn    func(_ cache.Interface, target string) (*virt.File, error)
}

func (g *funcGenerator) Generate(cache cache.Interface, target string) (*virt.File, error) {
	return g.fn(cache, target)
}

func (g *funcGenerator) String() string {
	return g.label
}

func TestTreeSharedDirGenerator(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateDir(".", Func("a", func(_ cache.Interface, target string) (*virt.File, error) {
		is.NoErr(tree.GenerateFile("index.html", Func("b", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{
				Path: target,
				Data: []byte("<h1>index</h1>"),
			}, nil
		})))
		is.NoErr(tree.GenerateFile("about.html", Func("c", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{
				Path: target,
				Data: []byte("<h1>about</h1>"),
			}, nil
		})))
		return &virt.File{}, nil
	})))
	is.NoErr(tree.GenerateDir(".", Func("d", func(_ cache.Interface, target string) (*virt.File, error) {
		is.NoErr(tree.GenerateFile("favicon.ico", Func("e", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{
				Data: []byte("favicon.ico"),
			}, nil
		})))
		return &virt.File{}, nil
	})))
	match, ok := tree.FindPrefix("index.html")
	is.True(ok)
	is.Equal(match.Path, ".")
	is.True(match.Mode.IsGenDir())
	is.Equal(tree.Print(), `. mode=dg generators=a,d
`)
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
	is.Equal(tree.Print(), `. mode=dg generators=a,d
├── about.html mode=-g generators=c
├── favicon.ico mode=-g generators=e
└── index.html mode=-g generators=b
`)
}

func TestTreeSharedDirAndDirGenerator(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateFile("favicon.ico", Func("a", func(_ cache.Interface, target string) (*virt.File, error) {
		return &virt.File{
			Path: target,
			Data: []byte("favicon.ico"),
		}, nil
	})))
	is.NoErr(tree.GenerateDir(".", Func("b", func(_ cache.Interface, target string) (*virt.File, error) {
		is.NoErr(tree.GenerateFile("index.html", Func("c", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{
				Data: []byte("<h1>index</h1>"),
			}, nil
		})))
		is.NoErr(tree.GenerateFile("about.html", Func("c", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{
				Data: []byte("<h1>about</h1>"),
			}, nil
		})))
		return &virt.File{}, nil
	})))
	is.NoErr(tree.GenerateDir(".", Func("c", func(_ cache.Interface, target string) (*virt.File, error) {
		is.NoErr(tree.GenerateDir("random_dir", Func("d", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{}, nil
		})))
		is.NoErr(tree.GenerateFile("about.js", Func("e", func(_ cache.Interface, target string) (*virt.File, error) {
			return &virt.File{
				Data: []byte("console.log('about')"),
			}, nil
		})))
		return &virt.File{}, nil
	})))
	match, ok := tree.FindPrefix("index.html")
	is.True(ok)
	is.Equal(match.Path, ".")
	is.True(match.Mode.IsGenDir())
	expect := `. mode=dg generators=b,c
└── favicon.ico mode=-g generators=a
`
	is.Equal(tree.Print(), expect)
	vfile, err := match.Generate(nil, ".")
	is.NoErr(err)
	is.Equal(vfile.Path, ".")
	is.True(vfile.Mode.IsDir())
	is.Equal(len(vfile.Entries), 5)
	is.Equal(vfile.Entries[0].Name(), "about.html")
	is.True(!vfile.Entries[0].IsDir())
	is.Equal(vfile.Entries[1].Name(), "about.js")
	is.True(!vfile.Entries[1].IsDir())
	is.Equal(vfile.Entries[2].Name(), "favicon.ico")
	is.True(!vfile.Entries[2].IsDir())
	is.Equal(vfile.Entries[3].Name(), "index.html")
	is.True(!vfile.Entries[3].IsDir())
	is.Equal(vfile.Entries[4].Name(), "random_dir")
	is.True(vfile.Entries[4].IsDir())
}

func TestTreeGenDirFileGenOverride(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateDir("a", ag))
	is.NoErr(tree.GenerateDir("a", bg))
	err := tree.GenerateFile("a", cg)
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), "already a directory"))
	match, ok := tree.Find("a")
	is.True(ok)
	is.Equal(match.Path, "a")
	is.True(match.Mode.IsGen())
	expect := `. mode=d-
└── a mode=dg generators=a,b
`
	is.Equal(tree.Print(), expect)
}

func TestTreeFileGenOverride(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateFile("a.txt", Func("a", func(_ cache.Interface, target string) (*virt.File, error) {
		return &virt.File{
			Path: target,
			Data: []byte("1"),
		}, nil
	})))
	is.NoErr(tree.GenerateFile("a.txt", Func("b", func(_ cache.Interface, target string) (*virt.File, error) {
		return &virt.File{
			Path: target,
			Data: []byte("2"),
		}, nil
	})))
	match, ok := tree.Find("a.txt")
	is.True(ok)
	is.Equal(match.Path, "a.txt")
	is.True(match.Mode.IsGen())
	expect := `. mode=d-
└── a.txt mode=-g generators=b
`
	is.Equal(tree.Print(), expect)
	vfile, err := match.Generate(nil, "a.txt")
	is.NoErr(err)
	is.Equal(vfile.Path, "a.txt")
	is.True(vfile.Mode.IsRegular())
	is.Equal(string(vfile.Data), "2")
}

func TestDynamic(t *testing.T) {
	is := is.New(t)
	tree := tree.New()
	is.NoErr(tree.GenerateDir("bud", Func("a", func(_ cache.Interface, target string) (*virt.File, error) {
		err := tree.GenerateDir("bud/docs", Func("b", func(_ cache.Interface, target string) (*virt.File, error) {
			err := tree.GenerateFile("bud/docs/a.txt", Func("c", func(_ cache.Interface, target string) (*virt.File, error) {
				return &virt.File{
					Path: target,
					Data: []byte("1"),
				}, nil
			}))
			return &virt.File{}, err
		}))
		return &virt.File{}, err
	})))
	match, ok := tree.FindPrefix("bud/docs/a.txt")
	is.True(ok)
	is.Equal(match.Path, "bud")
	vfile, err := match.Generate(nil, "bud/docs/a.txt")
	is.NoErr(err)
	is.Equal(vfile.Path, "bud")
	is.True(vfile.Mode.IsDir())
	is.Equal(len(vfile.Entries), 1)
	is.Equal(vfile.Entries[0].Name(), "docs")
	is.True(vfile.Entries[0].IsDir())

	// Try again now that we've discovered bud/docs
	match, ok = tree.FindPrefix("bud/docs/a.txt")
	is.True(ok)
	is.Equal(match.Path, "bud/docs")
	vfile, err = match.Generate(nil, "bud/docs/a.txt")
	is.NoErr(err)
	is.Equal(vfile.Path, "bud/docs")
	is.True(vfile.Mode.IsDir())
	is.Equal(len(vfile.Entries), 1)
	is.Equal(vfile.Entries[0].Name(), "a.txt")
	is.True(!vfile.Entries[0].IsDir())

	// Try again now that we've discovered bud/docs/a.txt
	match, ok = tree.Find("bud/docs/a.txt")
	is.True(ok)
	is.Equal(match.Path, "bud/docs/a.txt")
	is.True(match.Mode.IsGen())
	vfile, err = match.Generate(nil, "bud/docs/a.txt")
	is.NoErr(err)
	is.Equal(vfile.Path, "bud/docs/a.txt")
	is.True(vfile.Mode.IsRegular())
	is.Equal(string(vfile.Data), "1")
	is.Equal(len(vfile.Entries), 0)

	is.Equal(tree.Print(), `. mode=d-
└── bud mode=dg generators=a
    └── docs mode=dg generators=b
        └── a.txt mode=-g generators=c
`)
}
