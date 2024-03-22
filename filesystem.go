package genfs

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/matthewmueller/genfs/internal/cache"
	"github.com/matthewmueller/genfs/internal/tree"
	"github.com/matthewmueller/virt"
)

type FileSystem struct {
	fsys fs.FS
	tree *tree.Tree
}

var _ fs.FS = (*FileSystem)(nil)
var _ fs.ReadDirFS = (*FileSystem)(nil)

func (f *FileSystem) GenerateFile(relpath string, fn func(fsys FS, file *File) error) error {
	dir := &Dir{f, f.tree, relpath, ".", fs.ModeDir}
	return dir.GenerateFile(relpath, fn)
}

func (f *FileSystem) FileGenerator(relpath string, generator FileGenerator) error {
	dir := &Dir{f, f.tree, relpath, ".", fs.ModeDir}
	return dir.FileGenerator(relpath, generator)
}

func (f *FileSystem) GenerateDir(reldir string, fn func(fsys FS, dir *Dir) error) error {
	dir := &Dir{f, f.tree, reldir, ".", fs.ModeDir}
	return dir.GenerateDir(reldir, fn)
}

func (f *FileSystem) DirGenerator(reldir string, generator DirGenerator) error {
	dir := &Dir{f, f.tree, reldir, ".", fs.ModeDir}
	return dir.DirGenerator(reldir, generator)
}

func (f *FileSystem) Open(name string) (fs.File, error) {
	return f.openWith(cache.Discard(), name)
}

func (f *FileSystem) openWith(cache cache.Interface, target string) (fs.File, error) {
	return f.open(cache, "", target)
}

// ReadDir reads the named directory. We implement ReadDir in addition to Open
// so that we can merge generated files with the fs.FS files that can later be
// read by Open.
func (f *FileSystem) ReadDir(name string) (des []fs.DirEntry, err error) {
	return f.readDirWith(cache.Discard(), name)
}

func (f *FileSystem) readDirWith(cache cache.Interface, name string) (entries []fs.DirEntry, err error) {
	found := false

	// First try finding an exact match, generate the directory, and append its
	// entries
	if match, ok := f.tree.Find(name); ok && match.Mode.IsDir() {
		if vfile, err := match.Generate(cache, name); err == nil {
			for _, entry := range vfile.Entries {
				entries = append(entries, wrapEntry(f, entry))
			}
			found = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("readdir: error generating directory %q: %w", name, err)
		}
	}

	// Next try reading the directory from the fallback filesystem and append its
	// entries
	if des, err := fs.ReadDir(f.fsys, name); err == nil {
		entries = append(entries, des...)
		found = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("readdir: error reading directory %q: %w", name, err)
	}

	// If we didn't find anything, return fs.ErrNotExist
	if !found {
		return nil, fmt.Errorf("readdir: %q: %w", name, fs.ErrNotExist)
	}

	return dirEntrySet(entries), nil
}

func dirEntrySet(entries []fs.DirEntry) (des []fs.DirEntry) {
	seen := map[string]bool{}
	for _, entry := range entries {
		if seen[entry.Name()] {
			continue
		}
		seen[entry.Name()] = true
		des = append(des, entry)
	}
	sort.Slice(des, func(i, j int) bool {
		return des[i].Name() < des[j].Name()
	})
	return des
}

func (f *FileSystem) open(cache cache.Interface, previous, target string) (fs.File, error) {
	// Check that target is valid
	if !fs.ValidPath(target) {
		return nil, &fs.PathError{
			Op:   "open",
			Path: target,
			Err:  fs.ErrInvalid,
		}
	}

	// First try finding an exact match
	match, ok := f.tree.Find(target)
	if ok && match.Mode.IsGen() {
		if vfile, err := match.Generate(cache, target); err == nil {
			return wrapFile(f, vfile.Path, virt.Open(vfile)), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("vtree: error generating %q: %w", target, err)
		}
	}

	// Next try opening the file from the fallback filesystem
	if file, err := f.fsys.Open(target); err == nil {
		return wrapFile(f, target, file), nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("vtree: error opening %q: %w", target, err)
	}

	// Next, if we did find a match above, but it's not a generator, it must be
	// a filler directory, so return it now
	if ok && match.Mode.IsDir() {
		vfile, err := match.Generate(cache, target)
		if err != nil {
			return nil, fmt.Errorf("vtree: error generating directory %q: %w", target, err)
		}
		return wrapFile(f, vfile.Path, virt.Open(vfile)), nil
	}

	// Lastly, try finding a node by its prefix. We only allow directory
	// generators because they can generate sub-files and directories that will
	// end up matching.
	match, ok = f.tree.FindPrefix(target)
	if !ok || !match.Mode.IsGenDir() {
		return nil, fmt.Errorf("vtree: %q %w", target, fs.ErrNotExist)
	}

	// Ignore the generated file, because this isn't an exact match anyway
	if _, err := match.Generate(cache, target); err != nil {
		return nil, fmt.Errorf("vtree: error generating directory %q: %w", target, err)
	}

	// If we're not making progress, return an error
	if match.Path == previous {
		return nil, fmt.Errorf("vtree: %q: %w", target, fs.ErrNotExist)
	}

	// Now that the directory has been generated, try again
	return f.open(cache, match.Path, target)
}
