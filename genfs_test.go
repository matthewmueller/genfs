package genfs_test

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
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
	gfs := genfs.New()
	gfs.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("a"))
		return nil
	})
	code, err := fs.ReadFile(gfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestGenerateDir(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("docs", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte("a"))
				return nil
			})
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "bud/docs/a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

type tailwind struct {
}

func (t *tailwind) GenerateFile(fs genfs.FS, file *genfs.File) error {
	file.Write([]byte("/* tailwind */"))
	return nil
}

type svelte struct {
}

func (t *svelte) GenerateFile(fs genfs.FS, file *genfs.File) error {
	file.Write([]byte("/* svelte */"))
	return nil
}

func TestFS(t *testing.T) {
	is := is.New(t)
	gfs := genfs.New()
	gfs.FileGenerator("bud/public/tailwind/tailwind.css", &tailwind{})
	gfs.FileGenerator("bud/view/index.svelte", &svelte{})

	// .
	des, err := fs.ReadDir(gfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)

	// bud
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Mode(), fs.ModeDir)
	stat, err := fs.Stat(gfs, "bud")
	is.NoErr(err)
	is.Equal(stat.Mode(), fs.ModeDir)

	file, err := gfs.Open("bud")
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
	des, err = fs.ReadDir(gfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "public")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "public")
	stat, err = fs.Stat(gfs, "bud/public")
	is.NoErr(err)
	is.Equal(stat.Name(), "public")

	// return errors for non-existent files
	_, err = gfs.Open("bud\\public")
	is.True(errors.Is(err, fs.ErrNotExist))

	// bud/public/tailwind
	des, err = fs.ReadDir(gfs, "bud/public/tailwind")
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

	// read bfserated data
	data, err := fs.ReadFile(gfs, "bud/public/index.html")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.True(data == nil)
	data, err = fs.ReadFile(gfs, "bud/public/tailwind/tailwind.css")
	is.NoErr(err)
	is.Equal(string(data), "/* tailwind */")
	data, err = fs.ReadFile(gfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(data), "/* svelte */")

	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(gfs, "bud/public/tailwind/tailwind.css", "bud/view/index.svelte"))
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
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", view())

	// bud
	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)
	fi, err := des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "view")

	// bud/view
	stat, err := fs.Stat(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.IsDir(), true)
	is.Equal(stat.Mode(), fs.ModeDir)

	_, err = bfs.Open("about")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))

	_, err = bfs.Open("bud/view/.")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrInvalid))

	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h1>index</h1>")
	code, err = fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "<h2>about</h2>")

	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
	is.Equal(des[0].IsDir(), false)
	is.Equal(des[0].Type(), fs.FileMode(0))
	fi, err = des[0].Info()
	is.NoErr(err)
	is.Equal(fi.Name(), "about.svelte")
	is.Equal(fi.Mode(), fs.FileMode(0))
	stat, err = fs.Stat(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")

	is.NoErr(fstest.TestFS(bfs, "bud/view/index.svelte", "bud/view/about/about.svelte"))
}

func TestAll(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", view())

	// .
	file, err := bfs.Open(".")
	is.NoErr(err)
	rbfs, ok := file.(fs.ReadDirFile)
	is.True(ok)
	des, err := rbfs.ReadDir(-1)
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
	stat, err = fs.Stat(bfs, ".")
	is.NoErr(err)
	is.Equal(stat.Name(), ".")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir .
	des, err = fs.ReadDir(bfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "bud")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)

	// bud
	file, err = bfs.Open("bud")
	is.NoErr(err)
	rbfs, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rbfs.ReadDir(-1)
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
	stat, err = fs.Stat(bfs, "bud")
	is.NoErr(err)
	is.Equal(stat.Name(), "bud")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[0].Type(), fs.ModeDir)

	// bud/view
	file, err = bfs.Open("bud/view")
	is.NoErr(err)
	rbfs, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rbfs.ReadDir(-1)
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
	stat, err = fs.Stat(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(bfs, "bud/view")
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
	file, err = bfs.Open("bud/view/about")
	is.NoErr(err)
	rbfs, ok = file.(fs.ReadDirFile)
	is.True(ok)
	des, err = rbfs.ReadDir(-1)
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
	stat, err = fs.Stat(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(stat.Name(), "about")
	is.Equal(stat.Mode(), fs.ModeDir)
	is.True(stat.IsDir())
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(0))
	is.Equal(stat.Sys(), nil)
	// ReadDir bud
	des, err = fs.ReadDir(bfs, "bud/view/about")
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
	file, err = bfs.Open("bud/view/index.svelte")
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
	stat, err = fs.Stat(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// ReadFile
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)

	// bud/view/about/about.svelte
	// Open
	file, err = bfs.Open("bud/view/about/about.svelte")
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
	stat, err = fs.Stat(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.Name(), "about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(14))
	is.Equal(stat.Sys(), nil)
	// ReadFile
	code, err = fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h2>about</h2>`)

	// Run TestFS
	err = fstest.TestFS(bfs, "bud", "bud/view", "bud/view/index.svelte", "bud/view/about/about.svelte")
	is.NoErr(err)
}

func TestDir(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
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
	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "view")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[0].IsDir(), true)
	is.Equal(des[1].Name(), "users")
	is.Equal(des[1].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "me")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view/about/me")
	is.NoErr(err)
	is.Equal(len(des), 0)
	des, err = fs.ReadDir(bfs, "bud/view/users")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "admin")
	is.Equal(des[0].IsDir(), true)
	des, err = fs.ReadDir(bfs, "bud/view/users/admin")
	is.NoErr(err)
	is.Equal(len(des), 0)

	// Run TestFS
	err = fstest.TestFS(bfs, "bud/view/about/me", "bud/view/users/admin")
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
	bfs := genfs.New()
	sfs := bfs.Session()
	sfs.FS = fsys
	code, err := fs.ReadFile(sfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestGenerateFileError(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateFile("bud/main.go", func(fsys genfs.FS, file *genfs.File) error {
		return fs.ErrNotExist
	})
	code, err := fs.ReadFile(bfs, "bud/main.go")
	is.True(err != nil)
	is.True(strings.Contains(err.Error(), `genfs: open "bud/main.go"`))
	is.True(strings.Contains(err.Error(), `file does not exist`))
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
}

func TestHTTP(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile(dir.Relative(), func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(dir.Target() + "'s data"))
			return nil
		})
		return nil
	})
	hfs := http.FS(bfs)

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
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile("about/about.svelte", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(rootless(file.Target())))
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), "view/about/about.svelte")
}

func TestDynamicDir(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
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
	des, err := fs.ReadDir(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "about")
	is.Equal(des[1].Name(), "index.svelte")
	des, err = fs.ReadDir(bfs, "bud/view/about")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "about.svelte")
}

func TestBases(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		return nil
	})
	bfs.GenerateDir("bud/controller", func(fsys genfs.FS, dir *genfs.Dir) error {
		return nil
	})
	stat, err := fs.Stat(bfs, "bud/controller")
	is.NoErr(err)
	is.Equal(stat.Name(), "controller")
	stat, err = fs.Stat(bfs, "bud/view")
	is.NoErr(err)
	is.Equal(stat.Name(), "view")
}

func TestDirUnevenMerge(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("public", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateFile("favicon.ico", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte("cool_favicon.ico"))
				return nil
			})
			return nil
		})
		return nil
	})
	bfs.GenerateDir("bud", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateDir("controller", func(fsys genfs.FS, dir *genfs.Dir) error {
			dir.GenerateFile("controller.go", func(fsys genfs.FS, file *genfs.File) error {
				file.Write([]byte("package controller"))
				return nil
			})
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "bud/view/public/favicon.ico")
	is.NoErr(err)
	is.Equal(string(code), "cool_favicon.ico")
	code, err = fs.ReadFile(bfs, "bud/controller/controller.go")
	is.NoErr(err)
	is.Equal(string(code), "package controller")
}

// Add the view
func TestAddGenerator(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/view", view())

	// Add the controller
	bfs.GenerateDir("bud/controller", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.GenerateFile("controller.go", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`package controller`))
			return nil
		})
		return nil
	})

	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "controller")
	is.Equal(des[1].Name(), "view")

	// Read from view
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)

	// Read from controller
	code, err = fs.ReadFile(bfs, "bud/controller/controller.go")
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
	bfs := genfs.New()
	bfs.FileGenerator("bud/command/command.go", &commandGenerator{Input: "a"})
	code, err := fs.ReadFile(bfs, "bud/command/command.go")
	is.NoErr(err)
	is.Equal(string(code), "aa")
}

func TestDirGenerator(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.DirGenerator("bud/view", &commandGenerator{Input: "a"})
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), "aa")
}

func TestDotReadDirEmpty(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateFile("bud/bfserate/main.go", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("package main"))
		return nil
	})
	bfs.GenerateFile("go.mod", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("module pkg"))
		return nil
	})
	des, err := fs.ReadDir(bfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 2)
}

func TestEmbedOpen(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.FileGenerator("bud/view/index.svelte", &genfs.Embed{
		Data: []byte(`<h1>index</h1>`),
	})
	bfs.FileGenerator("bud/view/about/about.svelte", &genfs.Embed{
		Data: []byte(`<h1>about</h1>`),
	})
	bfs.FileGenerator("bud/public/favicon.ico", &genfs.Embed{
		Data: []byte(`favicon.ico`),
	})
	// bud/view/index.svelte
	code, err := fs.ReadFile(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>index</h1>`)
	stat, err := fs.Stat(bfs, "bud/view/index.svelte")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/view/about/about.svelte
	code, err = fs.ReadFile(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `<h1>about</h1>`)
	stat, err = fs.Stat(bfs, "bud/view/about/about.svelte")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/public/favicon.ico
	code, err = fs.ReadFile(bfs, "bud/public/favicon.ico")
	is.NoErr(err)
	is.Equal(string(code), `favicon.ico`)
	stat, err = fs.Stat(bfs, "bud/public/favicon.ico")
	is.NoErr(err)
	is.Equal(stat.ModTime(), time.Time{})
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)

	// bud/public
	des, err := fs.ReadDir(bfs, "bud/public")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "favicon.ico")
}

func TestGoModGoMod(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateFile("go.mod", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("module app.com\nrequire mod.test/module v1.2.4"))
		return nil
	})
	stat, err := fs.Stat(bfs, "go.mod/go.mod")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	stat, err = fs.Stat(bfs, "go.mod")
	is.NoErr(err)
	is.Equal(stat.Name(), "go.mod")
}

func TestGoModGoModEmbed(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.FileGenerator("go.mod", &genfs.Embed{
		Data: []byte("module app.com\nrequire mod.test/module v1.2.4"),
	})
	stat, err := fs.Stat(bfs, "go.mod/go.mod")
	is.True(err != nil)
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	stat, err = fs.Stat(bfs, "go.mod")
	is.NoErr(err)
	is.Equal(stat.Name(), "go.mod")
}

func TestReadDirNotExists(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	reads := 0
	bfs.GenerateFile("bud/controller/controller.go", func(fsys genfs.FS, file *genfs.File) error {
		reads++
		return fs.ErrNotExist
	})
	// Generators aren't called on dirs, so the value is wrong until read or stat.
	des, err := fs.ReadDir(bfs, "bud/controller")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(reads, 0)
	code, err := fs.ReadFile(bfs, "bud/controller/controller.go")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
	is.Equal(reads, 1)
}

func TestReadRootNotExists(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	reads := 0
	bfs.GenerateFile("controller.go", func(fsys genfs.FS, file *genfs.File) error {
		reads++
		return fs.ErrNotExist
	})
	// Generators aren't called on dirs, so the value is wrong until read or stat.
	des, err := fs.ReadDir(bfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(reads, 0)
	code, err := fs.ReadFile(bfs, "controller.go")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(code, nil)
	is.Equal(reads, 1)
}

func TestServeFile(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.ServeFile("duo/view", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte(file.Target() + `'s data`))
		return nil
	})
	des, err := fs.ReadDir(bfs, "duo/view")
	is.NoErr(err)
	is.Equal(len(des), 0)

	// _index.svelte
	file, err := bfs.Open("duo/view/_index.svelte")
	is.NoErr(err)
	code, err := fs.ReadFile(bfs, "duo/view/_index.svelte")
	is.NoErr(err)
	is.Equal(string(code), `duo/view/_index.svelte's data`)
	stat, err := file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "_index.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(29))
	is.Equal(stat.Sys(), nil)

	// about/_about.svelte
	file, err = bfs.Open("duo/view/about/_about.svelte")
	is.NoErr(err)
	stat, err = file.Stat()
	is.NoErr(err)
	is.Equal(stat.Name(), "_about.svelte")
	is.Equal(stat.Mode(), fs.FileMode(0))
	is.Equal(stat.IsDir(), false)
	is.True(stat.ModTime().IsZero())
	is.Equal(stat.Size(), int64(35))
	is.Equal(stat.Sys(), nil)
	code, err = fs.ReadFile(bfs, "duo/view/about/_about.svelte")
	is.NoErr(err)
	is.Equal(string(code), `duo/view/about/_about.svelte's data`)
}

func TestGenerateDirNotExists(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("bud/public", func(fsys genfs.FS, dir *genfs.Dir) error {
		return fs.ErrNotExist
	})
	stat, err := fs.Stat(bfs, "bud/public")
	is.True(errors.Is(err, fs.ErrNotExist))
	is.Equal(stat, nil)
	des, err := fs.ReadDir(bfs, "bud/public")
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
	bfs := genfs.New()
	bfs.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	sfs := bfs.Session()
	sfs.FS = fsys
	code, err := fs.ReadFile(sfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "b")
}

func TestSideBySideRoot(t *testing.T) {
	is := is.New(t)
	fsys := virt.Tree{
		"a.txt": &virt.File{Data: []byte("a")},
	}
	gfs := genfs.New()
	gfs.GenerateFile("b.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	sfs := gfs.Session()
	sfs.FS = fsys
	des, err := fs.ReadDir(sfs, ".")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "a.txt")
	is.Equal(des[1].Name(), "b.txt")
	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(sfs, "a.txt", "b.txt"))
}

func TestSideBySideDir(t *testing.T) {
	is := is.New(t)
	fsys := virt.Tree{
		"app/a.txt": &virt.File{Data: []byte("a")},
	}
	gfs := genfs.New()
	gfs.GenerateFile("app/b.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	sfs := gfs.Session()
	sfs.FS = fsys
	des, err := fs.ReadDir(sfs, "app")
	is.NoErr(err)
	is.Equal(len(des), 2)
	is.Equal(des[0].Name(), "a.txt")
	is.Equal(des[1].Name(), "b.txt")
	// run the TestFS compliance test suite
	is.NoErr(fstest.TestFS(sfs, "app/a.txt", "app/b.txt"))
}

func TestCyclesOk(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		is.NoErr(fsys.Watch("a.txt"))
		file.Write([]byte("a"))
		return nil
	})
	code, err := fs.ReadFile(bfs, "a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
}

func TestDirServeFile(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateDir("service", func(fsys genfs.FS, dir *genfs.Dir) error {
		dir.ServeFile("transform", func(fsys genfs.FS, file *genfs.File) error {
			file.Write([]byte(`transforming: ` + file.Relative()))
			return nil
		})
		return nil
	})
	code, err := fs.ReadFile(bfs, "service/transform/a.txt")
	is.NoErr(err)
	is.Equal(string(code), "transforming: a.txt")
	code, err = fs.ReadFile(bfs, "service/transform/b/b.txt")
	is.NoErr(err)
	is.Equal(string(code), "transforming: b/b.txt")
}

func TestSeek(t *testing.T) {
	is := is.New(t)
	gfs := genfs.New()
	gfs.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("ab"))
		return nil
	})
	file, err := gfs.Open("a.txt")
	is.NoErr(err)
	defer file.Close()
	seeker, ok := file.(io.Seeker)
	is.True(ok)
	n, err := seeker.Seek(1, io.SeekStart)
	is.NoErr(err)
	is.Equal(n, int64(1))
	code, err := io.ReadAll(file)
	is.NoErr(err)
	is.Equal(string(code), "b")
}

func TestExternal(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	fsys := &virt.List{}
	bfs.GenerateDir("bud", func(_ genfs.FS, dir *genfs.Dir) error {
		dir.GenerateExternal("app", func(_ genfs.FS, file *genfs.External) error {
			is.Equal(file.Target(), "bud/app")
			return fsys.WriteFile(file.Target(), []byte("my app"), 0644)
		})
		return nil
	})
	is.NoErr(virt.SyncFS(bfs, fsys, "."))
	code, err := fs.ReadFile(fsys, "bud/app")
	is.NoErr(err)
	is.Equal(string(code), "my app")
}

func TestDirMount(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	mount := genfs.New()
	called := map[string]int{}
	mount.GenerateFile("tailwind/tailwind.go", func(fsys genfs.FS, file *genfs.File) error {
		called[file.Path()]++
		file.Write([]byte("package tailwind"))
		return nil
	})
	mount.GenerateFile("html/html.go", func(fsys genfs.FS, file *genfs.File) error {
		called[file.Path()]++
		file.Write([]byte("package html"))
		return nil
	})
	mount.GenerateFile("service.json", func(fsys genfs.FS, file *genfs.File) error {
		called[file.Path()]++
		file.Write([]byte(`{"name":"service"}`))
		return nil
	})
	bfs.GenerateDir("bud/generator", func(fsys genfs.FS, dir *genfs.Dir) error {
		called[dir.Path()]++
		return dir.Mount(mount)
	})

	des, err := fs.ReadDir(bfs, "bud")
	is.NoErr(err)
	is.Equal(len(des), 1)
	is.Equal(des[0].Name(), "generator")
	stat, err := des[0].Info()
	is.NoErr(err)
	is.True(stat.IsDir())
	is.Equal(stat.Name(), "generator")

	generator, err := bfs.Open("bud/generator")
	is.NoErr(err)
	fi, err := generator.Stat()
	is.NoErr(err)
	is.Equal(fi.Name(), "generator")
	is.True(fi.IsDir())
	is.NoErr(generator.Close())

	// Read the dir that mounts the filesystem
	des, err = fs.ReadDir(bfs, "bud/generator")
	is.NoErr(err)
	is.Equal(len(des), 3)

	// No mounted generators should have been called despite walking the mounted fs
	is.Equal(called["bud/generator"], 3)        // bud/generator
	is.Equal(called["tailwind/tailwind.go"], 0) // tailwind/tailwind.go
	is.Equal(called["html/html.go"], 0)         // html/html.go
	is.Equal(called["service.json"], 0)         // service.json

	// Read a virtual dir within the mounted dir
	des, err = fs.ReadDir(bfs, "bud/generator/tailwind")
	is.NoErr(err)
	is.Equal(len(des), 1)
	// No mounted generators should have been called
	is.Equal(called["bud/generator"], 3)        // bud/generator
	is.Equal(called["tailwind/tailwind.go"], 0) // tailwind/tailwind.go
	is.Equal(called["html/html.go"], 0)         // html/html.go
	is.Equal(called["service.json"], 0)         // service.json

	// Directly read a generated file within the mounted dir
	code, err := fs.ReadFile(bfs, "bud/generator/tailwind/tailwind.go")
	is.NoErr(err)
	is.Equal(string(code), "package tailwind")
	// Tailwind.go should have been called
	is.Equal(called["bud/generator"], 3)        // bud/generator
	is.Equal(called["tailwind/tailwind.go"], 1) // tailwind/tailwind.go
	is.Equal(called["html/html.go"], 0)         // html/html.go
	is.Equal(called["service.json"], 0)         // service.json

	is.NoErr(fstest.TestFS(bfs,
		"bud/generator/tailwind/tailwind.go",
		"bud/generator/html/html.go",
		"bud/generator/service.json",
	))
}

// Mounts have priority over generators. It probably should be the other way
// around, but it's not trivial to change so we'll avoid this situation for now.
func TestDirMountPriority(t *testing.T) {
	is := is.New(t)
	bfs := genfs.New()
	bfs.GenerateFile("bud/generator/service.json", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte(`{"name":"generator service"}`))
		return nil
	})
	bfs.GenerateDir("bud/generator", func(fsys genfs.FS, dir *genfs.Dir) error {
		return dir.Mount(&virt.Tree{
			"tailwind/tailwind.go": &virt.File{Data: []byte("package tailwind")},
			"html/html.go":         &virt.File{Data: []byte("package html")},
			"service.json":         &virt.File{Data: []byte(`{"name":"mount service"}`)},
		})
	})
	err := fstest.TestFS(bfs,
		"bud/generator/tailwind/tailwind.go",
		"bud/generator/html/html.go",
		"bud/generator/service.json",
	)
	is.NoErr(err)
	code, err := fs.ReadFile(bfs, "bud/generator/service.json")
	is.NoErr(err)
	is.Equal(string(code), `{"name":"mount service"}`)
}

func TestFilesWithinServe(t *testing.T) {
	is := is.New(t)
	fsys := virt.Tree{
		"bud/a.txt": &virt.File{Data: []byte("a")},
	}
	bfs := genfs.New()
	bfs.GenerateFile("bud/b.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("b"))
		return nil
	})
	bfs.ServeFile("bud", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte(file.Relative()))
		return nil
	})
	bfs.GenerateFile("bud/c.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.Write([]byte("c"))
		return nil
	})
	sfs := bfs.Session()
	sfs.FS = fsys
	code, err := fs.ReadFile(sfs, "bud/a.txt")
	is.NoErr(err)
	is.Equal(string(code), "a")
	code, err = fs.ReadFile(sfs, "bud/b.txt")
	is.NoErr(err)
	is.Equal(string(code), "b")
	code, err = fs.ReadFile(sfs, "bud/c.txt")
	is.NoErr(err)
	is.Equal(string(code), "c")
	code, err = fs.ReadFile(sfs, "bud/d.txt")
	is.NoErr(err)
	is.Equal(string(code), "d.txt")
	code, err = fs.ReadFile(sfs, "bud/e/f.txt")
	is.NoErr(err)
	is.Equal(string(code), "e/f.txt")
}

func ExampleFS() {
	fsys := genfs.New()
	fsys.GenerateFile("a.txt", func(fsys genfs.FS, file *genfs.File) error {
		file.WriteString("a")
		return nil
	})
	code, _ := fs.ReadFile(fsys, "a.txt")
	fmt.Println(string(code))
	// Output: a
}
