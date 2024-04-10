package genfs_test

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/matryer/is"
	"github.com/matthewmueller/genfs"
	"github.com/matthewmueller/virt"
)

func TestGenerateFile(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("a"))
		return nil
	})
	code, err := fs.ReadFile(tree, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestGenerateDir(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("docs", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte("a"))
				return nil
			})
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(tree, "bud/docs/a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestSizeMismatch(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("bud/public/tailwind/tailwind.css", func(fs genfs.FS, file *genfs.File) error {
		file.Write([]byte("/* tailwind */"))
		return nil
	})

	file, err := tree.Open("bud/public/tailwind")
	is.NoErr(err)
	des, err := file.(fs.ReadDirFile).ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "tailwind.css")
	info, err := des[0].Info()
	is.NoErr(err)
	is.Equal(info.Size(), int64(14))

	des, err = fs.ReadDir(tree, "bud/public/tailwind")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "tailwind.css")
	info, err = des[0].Info()
	is.NoErr(err)
	is.Equal(info.Size(), int64(14))
	file, err = tree.Open("bud/public/tailwind/tailwind.css")
	is.NoErr(err)
	stat, err := file.Stat()
	is.NoErr(err)
	is.Equal(stat.Size(), int64(14))
	stat, err = fs.Stat(tree, "bud/public/tailwind/tailwind.css")
	is.NoErr(err)
	is.Equal(stat.Size(), int64(14))
}

func TestReadDirMultipleTimes(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("bud/public/tailwind/tailwind.css", func(fs genfs.FS, file *genfs.File) error {
		file.Write([]byte("/* tailwind */"))
		return nil
	})
	file, err := tree.Open("bud/public/tailwind")
	is.NoErr(err)
	defer file.Close()
	des, err := file.(fs.ReadDirFile).ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	des, err = file.(fs.ReadDirFile).ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 0)
}

func TestSeek(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("a.txt", func(fs genfs.FS, file *genfs.File) error {
		file.Write([]byte("ab"))
		return nil
	})
	code, err := fs.ReadFile(tree, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "ab")
	file, err := tree.Open("a.txt")
	is.NoErr(err)
	defer file.Close()
	seeker, ok := file.(io.Seeker)
	is.True(ok)
	n, err := seeker.Seek(1, io.SeekStart)
	is.NoErr(err)
	is.Equal(n, int64(1))
}

func TestFS(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("bud/public/tailwind/tailwind.css", func(fs genfs.FS, file *genfs.File) error {
		file.Write([]byte("/* tailwind */"))
		return nil
	})
	tree.GenerateFile("bud/view/index.svelte", func(fs genfs.FS, file *genfs.File) error {
		file.Write([]byte("/* svelte */"))
		return nil
	})

	// .
	des, err := fs.ReadDir(tree, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)

	// bud
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Mode(), fs.ModeDir)
	stat, err := fs.Stat(tree, "bud")
	is.NoErr(err)
	is.Equal(stat.Mode(), fs.ModeDir)

	file, err := tree.Open("bud")
	is.NoErr(err)
	dir, ok := file.(fs.ReadDirFile)
	is.True(ok)
	des, err = dir.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "public")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[1].Name(), "view")
	is.Equal(des[1].IsDir(), true)

	// bud/public
	des, err = fs.ReadDir(tree, "bud")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "public")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "public")
	stat, err = fs.Stat(tree, "bud/public")
	is.NoErr(err)
	is.Equal(stat.Name(), "public")

	// return errors for non-existent files
	_, err = tree.Open("bud\\public")
	is.True(errors.Is(err, fs.ErrNotExist))

	// bud/public/tailwind
	des, err = fs.ReadDir(tree, "bud/public/tailwind")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "tailwind.css")
	is.Equal(des[0].IsDir(), false)
	info, err := des[0].Info()
	is.NoErr(err)
	is.Equal(info.Name(), "tailwind.css")
	is.Equal(info.Mode(), fs.FileMode(0))
	is.Equal(info.IsDir(), false)
	is.Equal(info.Size(), int64(14))

	// read the data
	data, err := fs.ReadFile(tree, "bud/public/index.html")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.True(data == nil)
	data, err = fs.ReadFile(tree, "bud/public/tailwind/tailwind.css")
	is.NoErr(err)
	is.Equal(string(data), "/* tailwind */")
	data, err = fs.ReadFile(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(data), "/* svelte */")

	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(tree, "bud/public/tailwind/tailwind.css", "bud/view/index.svelte"))
}

func view() func(fsys genfs.FS, dir *genfs.Dir) error {
	return func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile("index.svelte", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`<h1>index</h1>`))
			return nil
		})
		dir.GenerateFile("about/about.svelte", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`<h2>about</h2>`))
			return nil
		})
		return nil
	}
}

func TestViewFS(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", view())

	// bud
	des, err := fs.ReadDir(tree, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "view")

	// bud/view
	stat, err := fs.Stat(tree, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.IsDir(), true)
	is.Equal(stat.Mode(), fs.ModeDir)

	_, err = tree.Open("about")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))

	_, err = tree.Open("bud/view/.")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrInvalid))

	code, err := fs.ReadFile(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h1>index</h1>")
	code, err = fs.ReadFile(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h2>about</h2>")

	des, err = fs.ReadDir(tree, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.Mode(), fs.FileMode(0))
	stat, err = fs.Stat(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")

	is.NoErr(fstest.TestFS(tree, "bud/view/index.svelte", "bud/view/about/about.svelte"))
}

func TestAll(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", view())

	// .
	file, err := tree.Open(".")
	is.NoErr(err)
	rtree, ok := file.(fs.ReadDirFile)
	is.True(ok)
	des, err := rtree.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Name(), "bud")
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	stat, err := file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), ".")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat .
	stat, err = fs.Stat(tree, ".")
	is.NoErr(err)
	is.Equal(stat.Name(), ".")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir .
	des, err = fs.ReadDir(tree, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)

	// bud
	file, err = tree.Open("bud")
	is.NoErr(err)
	rtree, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rtree.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Name(), "view")
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "bud")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat bud
	stat, err = fs.Stat(tree, "bud")
	is.NoErr(err)
	is.Equal(stat.Name(), "bud")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(tree, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)

	// bud/view
	file, err = tree.Open("bud/view")
	is.NoErr(err)
	rtree, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rtree.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about")
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	is.Equal(des[1].Name(), "index.svelte")
	is.Equal(des[1].IsDir(), false)
	is.Equal(des[1].Type(), fs.FileMode(0))
	fi, err = des[1].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "index.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat bud
	stat, err = fs.Stat(tree, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(tree, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about")
	is.Equal(fi.IsDir(), true)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.ModeDir)
	is.Equal(fi.Size(), int64(0))
	is.Equal(fi.Sys(), nil)
	is.Equal(des[1].Name(), "index.svelte")
	is.Equal(des[1].IsDir(), false)
	is.Equal(des[1].Type(), fs.FileMode(0))
	fi, err = des[1].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "index.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)

	// bud/view/about
	file, err = tree.Open("bud/view/about")
	is.NoErr(err)
	rtree, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rtree.ReadDir(-1)
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "about")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// Stat bud
	stat, err = fs.Stat(tree, "bud/view/about")
	is.NoErr(err)
	is.Equal(stat.Name(), "about")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(tree, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.IsDir(), false)
	is.True(fi.ModTime().IsZero())
	is.Equal(fi.Mode(), fs.FileMode(0))
	is.Equal(fi.Size(), int64(14))
	is.Equal(fi.Sys(), nil)

	// bud/view/index.svelte
	// Open
	file, err = tree.Open("bud/view/index.svelte")
	is.NoErr(err)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// Stat
	stat, err = fs.Stat(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// ReadFile
	code, err := fs.ReadFile(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)

	// bud/view/about/about.svelte
	// Open
	file, err = tree.Open("bud/view/about/about.svelte")
	is.NoErr(err)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// Stat
	stat, err = fs.Stat(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// ReadFile
	code, err = fs.ReadFile(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h2>about</h2>`)

	// Run TestFS
	err = fstest.TestFS(tree, "bud", "bud/view", "bud/view/index.svelte", "bud/view/about/about.svelte")
	is.NoErr(err)
}

func TestDir(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("about", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateDir("me", func(fsys genfs.FS, dir *genfs.Dir) error {
				return nil
			})
			return nil
		})
		dir.GenerateDir("users/admin", func(fsys genfs.FS, dir *genfs.Dir) error {
			return nil
		})
		return nil
	})
	des, err := fs.ReadDir(tree, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(tree, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[1].Name(), "users")
	is.Equal(des[1].IsDir(), true)
	des, err = fs.ReadDir(tree, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "me")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(tree, "bud/view/about/me")
	is.NoErr(err)
	is.Equal(len(des), 0)
	des, err = fs.ReadDir(tree, "bud/view/users")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "admin")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(tree, "bud/view/users/admin")
	is.NoErr(err)
	is.Equal(len(des), 0)

	// Run TestFS
	err = fstest.TestFS(tree, "bud/view/about/me", "bud/view/users/admin")
	is.NoErr(err)
}

func TestReadFsys(t *testing.T) {
	is := is.New(t)
	fsys := virt.List{
		&virt.File{
			Path: "a.txt",
			Data: []byte("a"),
		},
	}
	tree := genfs.New(fsys)
	code, err := fs.ReadFile(tree, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestGenerateFileError(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("bud/main.go", func(fsys genfs.FS, file *genfs.File) error {
		return fs.ErrNotExist
	})
	code, err := fs.ReadFile(tree, "bud/main.go")
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), `"bud/main.go"`))
	is.True(strings.Contains(err.Error(), `file does not exist`))
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
}

func TestHTTP(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile(dir.Relative(), func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(dir.Target() + "'s data"))
			return nil
		})
		return nil
	})
	hfs := http.FS(tree)

	handler := func(w http.ResponseWriter, r *http.Request) {
		file, err := hfs.Open(r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		stat, err := file.Stat()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Add("Content-Type", "text/javascript")
		http.ServeContent(w, r, r.URL.Path, stat.ModTime(), file)
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/bud/view/_index.svelte", nil)
	handler(w, r)

	response := w.Result()
	body, err := io.ReadAll(response.Body)
	is.NoErr(err)
	is.Equal(string(body), `bud/view/_index.svelte's data`)
	is.Equal(response.StatusCode, 200)
}

func rootless(fpath string) string {
	parts := strings.Split(fpath, string(filepath.Separator))
	return path.Join(parts[1:]...)
}

// Test inner file and rootless
func TestTargetPath(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile("about/about.svelte", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(rootless(file.Target())))
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "view/about/about.svelte")
}

func TestDynamicDir(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		doms := []string{"about/about.svelte", "index.svelte"}
		for _, dom := range doms {
			dom := dom
			dir.GenerateFile(dom, func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte(`<h1>` + dom + `</h1>`))
				return nil
			})
		}
		return nil
	})
	des, err := fs.ReadDir(tree, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[1].Name(), "index.svelte")
	des, err = fs.ReadDir(tree, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	code, err := fs.ReadFile(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h1>about/about.svelte</h1>")
}

func TestBases(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		return nil
	})
	tree.GenerateDir("bud/controller", func(fsys genfs.FS, dir *genfs.Dir) error {
		return nil
	})
	stat, err := fs.Stat(tree, "bud/controller")
	is.NoErr(err)
	is.Equal(stat.Name(), "controller")
	stat, err = fs.Stat(tree, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
}

func TestDirUnevenMerge(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("public", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateFile("favicon.ico", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte("cool_favicon.ico"))
				return nil
			})
			return nil
		})
		return nil
	})
	tree.GenerateDir("bud", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("controller", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateFile("controller.go", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte("package controller"))
				return nil
			})
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(tree, "bud/view/public/favicon.ico")
	is.NoErr(err)
	is.Equal(string(code), "cool_favicon.ico")
	code, err = fs.ReadFile(tree, "bud/controller/controller.go")
	is.NoErr(err)
	is.Equal(string(code), "package controller")
}

// Add the view
func TestAddGenerator(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/view", view())

	// Add the controller
	tree.GenerateDir("bud/controller", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile("controller.go", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`package controller`))
			return nil
		})
		return nil
	})

	des, err := fs.ReadDir(tree, "bud")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "controller")
	is.Equal(des[1].Name(), "view")

	// Read from view
	code, err := fs.ReadFile(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)

	// Read from controller
	code, err = fs.ReadFile(tree, "bud/controller/controller.go")
	is.NoErr(err)
	is.Equal(string(code), `package controller`)
}

type commandGenerator struct {
	Input string
}

func (c *commandGenerator) GenerateFile(fsys genfs.FS, file *genfs.File) error {
	file.Write([]byte(c.Input + c.Input))
	return nil
}

func (c *commandGenerator) GenerateDir(fsys genfs.FS, dir *genfs.Dir) error {
	dir.GenerateFile("index.svelte", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte(c.Input + c.Input))
		return nil
	})
	return nil
}

func TestFileGenerator(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.FileGenerator("bud/command/command.go", &commandGenerator{Input: "a"})
	code, err := fs.ReadFile(tree, "bud/command/command.go")
	is.NoErr(err)
	is.Equal(string(code), "aa")
}

func TestDirGenerator(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.DirGenerator("bud/view", &commandGenerator{Input: "a"})
	code, err := fs.ReadFile(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), "aa")
}

func TestDotReadDirEmpty(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("bud/treeerate/main.go", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("package main"))
		return nil
	})
	tree.GenerateFile("go.mod", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("module pkg"))
		return nil
	})
	des, err := fs.ReadDir(tree, ".")
	is.NoErr(err)
	is.Equal(len(des), 2)
}

func TestEmbedOpen(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.FileGenerator("bud/view/index.svelte", &genfs.Embed{
		Data: []byte(`<h1>index</h1>`),
	})
	tree.FileGenerator("bud/view/about/about.svelte", &genfs.Embed{
		Data: []byte(`<h1>about</h1>`),
	})
	tree.FileGenerator("bud/public/favicon.ico", &genfs.Embed{
		Data: []byte(`favicon.ico`),
	})
	// bud/view/index.svelte
	code, err := fs.ReadFile(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
	stat, err := fs.Stat(tree, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/view/about/about.svelte
	code, err = fs.ReadFile(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>about</h1>`)
	stat, err = fs.Stat(tree, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/public/favicon.ico
	code, err = fs.ReadFile(tree, "bud/public/favicon.ico")
	is.NoErr(err)
	is.Equal(string(code), `favicon.ico`)
	stat, err = fs.Stat(tree, "bud/public/favicon.ico")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/public
	des, err := fs.ReadDir(tree, "bud/public")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "favicon.ico")
}

func TestGoModGoMod(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateFile("go.mod", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("module app.com\nrequire mod.test/module v1.2.4"))
		return nil
	})
	stat, err := fs.Stat(tree, "go.mod/go.mod")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	stat, err = fs.Stat(tree, "go.mod")
	is.NoErr(err)
	is.Equal(stat.Name(), "go.mod")
}

func TestGoModGoModEmbed(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.FileGenerator("go.mod", &genfs.Embed{
		Data: []byte("module app.com\nrequire mod.test/module v1.2.4"),
	})
	stat, err := fs.Stat(tree, "go.mod/go.mod")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	stat, err = fs.Stat(tree, "go.mod")
	is.NoErr(err)
	is.Equal(stat.Name(), "go.mod")
}

func TestReadDirNotExists(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	reads := 0
	tree.GenerateFile("bud/controller/controller.go", func(fsys genfs.FS, file *genfs.File) error {
		reads++
		return fs.ErrNotExist
	})
	// Generators aren't called on dirs, so the value is wrong until read or stat.
	des, err := fs.ReadDir(tree, "bud/controller")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(reads, 0)
	code, err := fs.ReadFile(tree, "bud/controller/controller.go")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
	is.Equal(reads, 1)
}

func TestReadRootNotExists(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	reads := 0
	tree.GenerateFile("controller.go", func(fsys genfs.FS, file *genfs.File) error {
		reads++
		return fs.ErrNotExist
	})
	// Generators aren't called on dirs, so the value is wrong until read or stat.
	des, err := fs.ReadDir(tree, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(reads, 0)
	code, err := fs.ReadFile(tree, "controller.go")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
	is.Equal(reads, 1)
}

func TestGenerateDirNotExists(t *testing.T) {
	is := is.New(t)
	fsys := virt.Map{}
	tree := genfs.New(fsys)
	tree.GenerateDir("bud/public", func(fsys genfs.FS, dir *genfs.Dir) error {
		return fs.ErrNotExist
	})
	stat, err := fs.Stat(tree, "bud/public")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	des, err := fs.ReadDir(tree, "bud/public")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(len(des), 0)
}

// Prioritize generators because they're in memory and quicker to determine if
// they're present in mergefs
func TestGeneratorPriority(t *testing.T) {
	is := is.New(t)
	fsys := virt.List{
		&virt.File{
			Path: "a.txt",
			Data: []byte("a"),
		},
	}
	tree := genfs.New(fsys)
	tree.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	code, err := fs.ReadFile(tree, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "b")
}

func TestSideBySideRoot(t *testing.T) {
	is := is.New(t)
	fsys := virt.Tree{
		"a.txt": &virt.File{Data: []byte("a")},
	}
	tree := genfs.New(fsys)
	tree.GenerateFile("b.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	des, err := fs.ReadDir(tree, ".")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "a.txt")
	is.Equal(des[1].Name(), "b.txt")
	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(tree, "a.txt", "b.txt"))
}

func TestSideBySideDir(t *testing.T) {
	is := is.New(t)
	fsys := virt.Tree{
		"app/a.txt": &virt.File{Data: []byte("a")},
	}
	tree := genfs.New(fsys)
	tree.GenerateFile("app/b.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	des, err := fs.ReadDir(tree, "app")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "a.txt")
	is.Equal(des[1].Name(), "b.txt")

	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(tree, "app/a.txt", "app/b.txt"))
}

func ExampleFS() {
	fsys := genfs.New(virt.Map{})
	fsys.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.WriteString("a")
		return nil
	})
	code, _ := fs.ReadFile(fsys, "a.txt")
	os.Stdout.Write(code)
	// Output: a
}

var favicon = []byte{0x00, 0x00, 0x01}

func TestDirShared(t *testing.T) {
	is := is.New(t)
	fsys := genfs.New(virt.Map{})
	fsys.GenerateDir(".", func(fsys genfs.FS, dir *genfs.Dir) error {
		switch dir.Target() {
		case "index.html":
			dir.GenerateFile("index.html", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte(`<h1>index</h1>`))
				return nil
			})
		case "index.js":
			dir.GenerateFile("index.js", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte(`console.log('index')`))
				return nil
			})
		}
		return nil
	})
	fsys.GenerateDir(".", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile("random.ico", func(fsys genfs.FS, file *genfs.File) error {
			file.Write(favicon)
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(fsys, "index.html")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
	code, err = fs.ReadFile(fsys, "index.js")
	is.NoErr(err)
	is.Equal(string(code), `console.log('index')`)
	code, err = fs.ReadFile(fsys, "random.ico")
	is.NoErr(err)
	is.Equal(code, favicon)
	des, err := fs.ReadDir(fsys, ".")
	is.NoErr(err)
	is.Equal(len(des), 3)
	is.Equal(des[0].Name(), "index.html")
	is.Equal(des[1].Name(), "index.js")
	is.Equal(des[2].Name(), "random.ico")
	is.NoErr(fstest.TestFS(fsys, "index.html", "index.js", "random.ico"))
}

func TestDirDuplicateLastWins(t *testing.T) {
	is := is.New(t)
	fsys := genfs.New(virt.Map{})
	fsys.GenerateDir(".", func(fsys genfs.FS, dir *genfs.Dir) error {
		return dir.GenerateFile("index.html", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`<h1>index</h1>`))
			return nil
		})
	})
	fsys.GenerateDir(".", func(fsys genfs.FS, dir *genfs.Dir) error {
		return dir.GenerateFile("index.html", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`<h1>index2</h1>`))
			return nil
		})
	})
	code, err := fs.ReadFile(fsys, "index.html")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index2</h1>`)
	is.NoErr(fstest.TestFS(fsys, "index.html"))
}

func TestSub(t *testing.T) {
	is := is.New(t)
	fsys := genfs.New(virt.Map{})
	fsys.GenerateFile("pages/index.html", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte(`<h1>index</h1>`))
		return nil
	})
	sub, err := fs.Sub(fsys, "pages")
	is.NoErr(err)
	code, err := fs.ReadFile(sub, "index.html")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
}

func TestGeneratedFileRetried(t *testing.T) {
	is := is.New(t)
	fsys := genfs.New(virt.Map{})
	called := 0
	innerCalled := 0
	fsys.GenerateDir("dist", func(fsys genfs.FS, dir *genfs.Dir) error {
		called++
		dir.GenerateFile("index.html", func(fsys genfs.FS, file *genfs.File) error {
			innerCalled++
			file.Write([]byte(`<h1>index</h1>`))
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(fsys, "dist/index.html")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
	code, err = fs.ReadFile(fsys, "dist/index.html")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
	is.Equal(called, 1)
	is.Equal(innerCalled, 2)
}
