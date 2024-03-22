package vtree

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/matthewmueller/genfs/internal/cache"
	"github.com/matthewmueller/virt"
	"github.com/xlab/treeprint"
)

type Generator interface {
	Generate(cache cache.Interface, target string) (*virt.File, error)
}

type FileGenerator interface {
	GenerateFile(fsys FS, file *File) error
}

type DirGenerator interface {
	GenerateDir(fsys FS, dir *Dir) error
}

type GeneratorFunc func(cache cache.Interface, target string) (*virt.File, error)

func (fn GeneratorFunc) Generate(cache cache.Interface, target string) (*virt.File, error) {
	return fn(cache, target)
}

type Embed struct {
	Data []byte
}

var _ FileGenerator = (*Embed)(nil)

func (e *Embed) GenerateFile(fsys FS, file *File) error {
	file.Write(e.Data)
	return nil
}

func New(fsys fs.FS) *Tree {
	return &Tree{
		fsys: fsys,
		root: &Node{
			Name:     ".",
			Mode:     ModeDir,
			children: map[string]*Node{},
		},
	}
}

type Tree struct {
	fsys fs.FS
	root *Node
}

var _ fs.FS = (*Tree)(nil)
var _ fs.ReadDirFS = (*Tree)(nil)

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

type Dir struct {
	tree   *Tree
	target string
	dir    string
	mode   fs.FileMode
}

func (d *Dir) Target() string {
	return d.target
}

func (d *Dir) Path() string {
	return d.dir
}

func (d *Dir) Mode() fs.FileMode {
	return d.mode
}

func (d *Dir) Relative() string {
	return relativePath(d.dir, d.target)
}

type FS fs.FS

func (d *Dir) GenerateFile(relpath string, fn func(fsys FS, file *File) error) error {
	return d.tree.GenerateFile2(path.Join(d.dir, relpath), GeneratorFunc(func(cache cache.Interface, target string) (*virt.File, error) {
		file := &File{target, relpath, fs.FileMode(0), &bytes.Buffer{}}
		fsys := scopedFS{d.tree, cache, relpath}
		if err := fn(fsys, file); err != nil {
			return nil, err
		}
		return &virt.File{
			Path: relpath,
			Mode: file.Mode(),
			Data: file.data.Bytes(),
		}, nil
	}))
}

func (d *Dir) FileGenerator(relpath string, generator FileGenerator) error {
	return d.GenerateFile(relpath, generator.GenerateFile)
}

func (d *Dir) GenerateDir(reldir string, fn func(fsys FS, dir *Dir) error) error {
	reldir = path.Join(d.dir, reldir)
	return d.tree.GenerateDir2(reldir, GeneratorFunc(func(cache cache.Interface, target string) (*virt.File, error) {
		dir := &Dir{d.tree, target, reldir, fs.ModeDir}
		fsys := scopedFS{d.tree, cache, reldir}
		if err := fn(fsys, dir); err != nil {
			return nil, err
		}
		return &virt.File{
			Path: reldir,
			Mode: dir.mode,
			// Intentionally nil, filled in by the tree
			Entries: nil,
		}, nil
	}))
}

func (d *Dir) DirGenerator(reldir string, generator DirGenerator) error {
	return d.GenerateDir(reldir, generator.GenerateDir)
}

func (t *Tree) GenerateFile(relpath string, fn func(fsys FS, file *File) error) error {
	dir := &Dir{t, relpath, ".", fs.ModeDir}
	return dir.GenerateFile(relpath, fn)
}

func (t *Tree) FileGenerator(relpath string, generator FileGenerator) error {
	dir := &Dir{t, relpath, ".", fs.ModeDir}
	return dir.FileGenerator(relpath, generator)
}

func (t *Tree) GenerateDir(reldir string, fn func(fsys FS, dir *Dir) error) error {
	dir := &Dir{t, reldir, ".", fs.ModeDir}
	return dir.GenerateDir(reldir, fn)
}

func (t *Tree) DirGenerator(reldir string, generator DirGenerator) error {
	dir := &Dir{t, reldir, ".", fs.ModeDir}
	return dir.DirGenerator(reldir, generator)
}

type scopedFS struct {
	fsys  fs.FS
	cache cache.Interface
	from  string
}

func (s scopedFS) Open(name string) (fs.File, error) {
	return s.fsys.Open(name)
}

func (t *Tree) openWith(cache cache.Interface, previous, target string) (fs.File, error) {
	// Check that target is valid
	if !fs.ValidPath(target) {
		return nil, &fs.PathError{
			Op:   "open",
			Path: target,
			Err:  fs.ErrInvalid,
		}
	}

	// First try finding an exact match
	match, ok := t.Find(target)
	if ok && match.Mode.IsGen() {
		if vfile, err := match.Generate(cache, target); err == nil {
			return wrapFile(t, vfile.Path, virt.Open(vfile)), nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("vtree: error generating %q: %w", target, err)
		}
	}

	// Next try opening the file from the fallback filesystem
	if file, err := t.fsys.Open(target); err == nil {
		return wrapFile(t, target, file), nil
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
		return wrapFile(t, vfile.Path, virt.Open(vfile)), nil
	}

	// Lastly, try finding a node by its prefix. We only allow directory
	// generators because they can generate sub-files and directories that will
	// end up matching.
	match, ok = t.FindPrefix(target)
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
	return t.openWith(cache, match.Path, target)
}

func (t *Tree) OpenWith(cache cache.Interface, target string) (fs.File, error) {
	return t.openWith(cache, "", target)
}

func (t *Tree) ReadDirWith(cache cache.Interface, name string) (entries []fs.DirEntry, err error) {
	found := false

	// First try finding an exact match, generate the directory, and append its
	// entries
	if match, ok := t.Find(name); ok && match.Mode.IsDir() {
		if vfile, err := match.Generate(cache, name); err == nil {
			for _, entry := range vfile.Entries {
				entries = append(entries, wrapEntry(t, entry))
			}
			found = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("readdir: error generating directory %q: %w", name, err)
		}
	}

	// Next try reading the directory from the fallback filesystem and append its
	// entries
	if des, err := fs.ReadDir(t.fsys, name); err == nil {
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

func (t *Tree) Open(name string) (fs.File, error) {
	return t.OpenWith(cache.Discard(), name)
}

// ReadDir reads the named directory. We implement ReadDir in addition to Open
// so that we can merge generated files with the fs.FS files that can later be
// read by Open.
func (t *Tree) ReadDir(name string) (des []fs.DirEntry, err error) {
	return t.ReadDirWith(cache.Discard(), name)
}

func (t *Tree) GenerateFile2(fpath string, generator Generator) error {
	fpath = path.Clean(fpath)
	if fpath == "." {
		return &fs.PathError{
			Op:   "GenerateFile",
			Path: fpath,
			Err:  fmt.Errorf("%w: path is already a directory", fs.ErrInvalid),
		}
	}
	// make the directory
	dir := path.Dir(fpath)
	parent, err := t.mkdirAll(dir)
	if err != nil {
		return err
	}
	name := path.Base(fpath)
	child, ok := parent.children[name]
	if !ok {
		// create the file generator
		parent.children[name] = &Node{
			Name:       name,
			Mode:       ModeGen,
			Generators: []Generator{generator},
		}
		return nil
	}
	switch child.Mode {
	case ModeGen:
		// Override the generator
		child.Generators = []Generator{generator}
		return nil
	default:
		return &fs.PathError{
			Op:   "GenerateDir",
			Path: fpath,
			Err:  fmt.Errorf("%w: path is already a directory", fs.ErrInvalid),
		}
	}
}

func (t *Tree) GenerateDir2(fpath string, generator Generator) error {
	fpath = path.Clean(fpath)
	// Turn the root into a dir generator
	if fpath == "." {
		t.root.Mode |= ModeGen
		t.root.Generators = append(t.root.Generators, generator)
		return nil
	}
	// make the directory
	dir := path.Dir(fpath)
	parent, err := t.mkdirAll(dir)
	if err != nil {
		return err
	}
	name := path.Base(fpath)
	child, ok := parent.children[name]
	if !ok {
		// create the directory generator
		parent.children[name] = &Node{
			Name:       name,
			Mode:       ModeGen | ModeDir,
			Generators: []Generator{generator},
			children:   map[string]*Node{},
		}
		return nil
	}
	switch child.Mode {
	case ModeGenDir:
		child.Generators = append(child.Generators, generator)
		return nil
	case ModeDir:
		child.Mode |= ModeGen
		child.Generators = append(child.Generators, generator)
		return nil
	default:
		return &fs.PathError{
			Op:   "GenerateDir",
			Path: fpath,
			Err:  fmt.Errorf("%w: path is already a file", fs.ErrInvalid),
		}
	}
}

func (t *Tree) FindPrefix(fpath string) (*Match, bool) {
	fpath = path.Clean(fpath)
	if fpath == "." {
		return &Match{
			Path:       ".",
			Mode:       t.root.Mode,
			generators: t.root.Generators,
			node:       t.root,
		}, true
	}
	segments := strings.Split(fpath, "/")
	node, remaining := t.root.findPrefix(segments)
	// Nodes that aren't dirs must be an exact match
	if len(remaining) > 0 && !node.Mode.IsDir() {
		return nil, false
	}
	prefix := strings.Join(segments[:len(segments)-len(remaining)], "/")
	return &Match{
		Path:       path.Clean(prefix),
		Mode:       node.Mode,
		generators: node.Generators,
		node:       node,
	}, true
}

func (t *Tree) Find(fpath string) (*Match, bool) {
	fpath = path.Clean(fpath)
	if fpath == "." {
		return &Match{
			Path:       ".",
			Mode:       t.root.Mode,
			generators: t.root.Generators,
			node:       t.root,
		}, true
	}
	segments := strings.Split(fpath, "/")
	node, ok := t.root.find(segments)
	if !ok {
		return nil, false
	}
	return &Match{
		Path:       fpath,
		Mode:       node.Mode,
		generators: node.Generators,
		node:       node,
	}, true
}

type Match struct {
	Path       string
	Mode       Mode
	generators []Generator
	node       *Node
}

func (m *Match) entries() (entries []*virt.DirEntry) {
	for _, child := range m.node.children {
		entries = append(entries, &virt.DirEntry{
			Path: path.Join(m.Path, child.Name),
			Mode: child.Mode.FileMode(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries
}

func (m *Match) Generate(cache cache.Interface, target string) (*virt.File, error) {
	switch m.Mode {
	case ModeGenDir:
		return m.generateGenDir(cache, target)
	case ModeGen:
		return m.generateGen(cache, target)
	case ModeDir:
		return m.generateDir(cache, target)
	default:
		return nil, fmt.Errorf("%w: invalid mode %s", fs.ErrInvalid, m.Mode)
	}
}

func (m *Match) generateGenDir(cache cache.Interface, target string) (*virt.File, error) {
	// Run generators to discover new files, but ignore their entries since they
	// shouldn't be creating entries anyway
	for i := len(m.generators) - 1; i >= 0; i-- {
		if _, err := m.generators[i].Generate(cache, target); err != nil {
			return nil, err
		}
	}
	// Return the directory with the children filled in
	return &virt.File{
		Path:    m.Path,
		Mode:    m.Mode.FileMode(),
		Entries: m.entries(),
	}, nil
}

func (m *Match) generateGen(cache cache.Interface, target string) (*virt.File, error) {
	// There should only be one generator for a file
	if len(m.generators) != 1 {
		return nil, fmt.Errorf("%w: expected 1 generator, got %d", fs.ErrInvalid, len(m.generators))
	}
	return m.generators[0].Generate(cache, target)
}

func (m *Match) generateDir(_ cache.Interface, _ string) (*virt.File, error) {
	// This is simply a filler directory created by mkdirAll, just return the
	// children
	return &virt.File{
		Path:    m.Path,
		Mode:    m.Mode.FileMode(),
		Entries: m.entries(),
	}, nil
}

func (t *Tree) Print() string {
	tp := treeprint.NewWithRoot(t.root.Format())
	t.root.Print(tp)
	return tp.String()
}

func (t *Tree) Delete(fpath string) {
	fpath = path.Clean(fpath)
	if fpath == "." {
		// Reset the root
		t.root = &Node{
			Name:     ".",
			Mode:     ModeDir,
			children: map[string]*Node{},
		}
		return
	}
	segments := strings.Split(fpath, "/")
	t.root.delete(segments)
}

type Node struct {
	Name       string
	Mode       Mode
	Generators []Generator
	children   map[string]*Node
}

func (t *Tree) mkdirAll(dir string) (node *Node, err error) {
	if dir == "." {
		return t.root, nil
	}
	segments := strings.Split(dir, "/")
	return t.root.mkdirAll(segments)
}

func (n *Node) mkdirAll(segments []string) (*Node, error) {
	if len(segments) == 0 {
		return n, nil
	}
	next := segments[0]
	child, ok := n.children[next]
	if !ok {
		n.children[next] = &Node{
			Name:     next,
			Mode:     ModeDir,
			children: map[string]*Node{},
		}
		return n.children[next].mkdirAll(segments[1:])
	}
	if !child.Mode.IsDir() {
		return nil, &fs.PathError{
			Op:   "mkdirAll",
			Path: strings.Join(segments, "/"),
			Err:  fmt.Errorf("%w: path is already a file", fs.ErrInvalid),
		}
	}
	return child.mkdirAll(segments[1:])
}

func (n *Node) find(segments []string) (*Node, bool) {
	if len(segments) == 0 {
		return n, true
	}
	next := segments[0]
	child, ok := n.children[next]
	if !ok {
		return nil, false
	}
	return child.find(segments[1:])
}

func (n *Node) findPrefix(segments []string) (*Node, []string) {
	if len(segments) == 0 {
		return n, segments
	}
	next := segments[0]
	child, ok := n.children[next]
	if !ok {
		return n, segments
	}
	return child.findPrefix(segments[1:])
}

func (n *Node) delete(segments []string) {
	next := segments[0]
	child, ok := n.children[next]
	if !ok {
		return
	}
	if len(segments) == 1 {
		delete(n.children, next)
		return
	}
	child.delete(segments[1:])
}

func (n *Node) Format() string {
	s := new(strings.Builder)
	s.WriteString(fmt.Sprintf("%s mode=%s", n.Name, n.Mode))
	if len(n.Generators) > 0 {
		generators := make([]string, len(n.Generators))
		for i, generator := range n.Generators {
			generators[i] = fmt.Sprintf("%v", generator)
		}
		s.WriteString(fmt.Sprintf(" generators=%s", strings.Join(generators, ",")))
	}
	return s.String()
}

func (n *Node) Children() []*Node {
	var children []*Node
	for _, child := range n.children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})
	return children
}

func (n *Node) Print(tree treeprint.Tree) string {
	for _, child := range n.Children() {
		branch := tree.AddBranch(child.Format())
		child.Print(branch)
	}
	return n.Name
}

func relativePath(base, target string) string {
	rel := strings.TrimPrefix(target, base)
	if rel == "" {
		return "."
	} else if rel[0] == '/' {
		rel = rel[1:]
	}
	return rel
}
