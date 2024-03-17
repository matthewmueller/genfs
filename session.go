package genfs

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/matthewmueller/virt"
)

// Session is a virtual filesystem session that supports caching and an
// underlying filesystem.
type Session struct {
	Cache Cache
	FS    fs.FS
	tree  *tree
}

func (f *Session) Open(target string) (fs.File, error) {
	// Check that target is valid
	if !fs.ValidPath(target) {
		return nil, formatError(fs.ErrInvalid, "invalid target path %q", target)
	}
	return f.openFrom("", target)
}

func (f *Session) openFrom(previous string, target string) (fs.File, error) {
	// First look for an exact matching generator
	node, found := f.tree.Find(target)
	if found && node.Generator != nil {
		file, err := node.Generator.Generate(f.Cache, target)
		if err != nil {
			return nil, formatError(err, "open %q", target)
		}
		return wrapFile(file, f, node.Path), nil
	}
	// Next try opening the file from the fallback filesystem
	if file, err := f.FS.Open(target); nil == err {
		return wrapFile(file, f, target), nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, formatError(err, "open %q", target)
	}
	// Next, if we did find a generator node above, return it now. It'll be a
	// filler directory, not a generator.
	if found && node.Mode.IsDir() {
		dir := virt.Open(&virt.File{
			Path: target,
			Mode: node.Mode.FileMode(),
		})
		return wrapFile(dir, f, node.Path), nil
	}
	// Lastly, try finding a node by its prefix
	node, found = f.tree.FindPrefix(target)
	if found && node.Path != previous && node.Mode.IsDir() && node.Generator != nil {
		if file, err := node.Generator.Generate(f.Cache, target); nil == err {
			return wrapFile(file, f, node.Path), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, formatError(err, "open by prefix %q", target)
		}
	}
	// Return a file not found error
	return nil, formatError(fs.ErrNotExist, "open %q", target)
}

func (f *Session) ReadDir(target string) ([]fs.DirEntry, error) {
	deset := newDirEntrySet()
	node, ok := f.tree.Find(target)
	if ok {
		if !node.Mode.IsDir() {
			return nil, formatError(errNotImplemented, "tree readdir %q", target)
		}
		// Run the directory generator
		if node.Mode.IsGen() {
			// Generate is expected to update the tree, that's why we don't use the
			// returned file
			if _, err := node.Generator.Generate(f.Cache, target); err != nil {
				return nil, err
			}
		}
		for _, child := range node.children {
			deset.Add(newDirEntry(f, child.Name, child.Mode.FileMode(), child.Path))
		}
	}
	des, err := fs.ReadDir(f.FS, target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, formatError(err, "fallback readdir %q", target)
	}
	for _, de := range des {
		deset.Add(de)
	}
	return deset.List(), nil
}

func formatError(err error, format string, args ...interface{}) error {
	return fmt.Errorf("genfs: %s. %w", fmt.Sprintf(format, args...), err)
}
