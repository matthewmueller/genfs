package vtree

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/matthewmueller/virt"
	"github.com/xlab/treeprint"
)

// FillerDir
// GenFile
// GenDir

// FillerDir => FillerDir do nothing d
// FillerDir => GenFile error
// FillerDir => GenDir append gd

// GenFile => GenFile error
// GenFile => GenDir error
// GenFile => FillerDir error

// GenDir => GenDir append gd
// GenDir => GenFile error
// GenDir => FillerDir do nothing gd

type Cache interface {
	Get(path string) (*virt.File, error)
	Set(path string, file *virt.File) error
	Link(from string, to ...string) error
}

type Generator interface {
	Generate(cache Cache, target string) (*virt.File, error)
}

func New() *Tree {
	return &Tree{
		root: &Node{
			Name:     ".",
			Mode:     ModeDir,
			children: map[string]*Node{},
		},
	}
}

type Tree struct {
	root *Node
}

func (t *Tree) GenerateFile(fpath string, generator Generator) error {
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

func (t *Tree) GenerateDir(fpath string, generator Generator) error {
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
	}, true
}

func (t *Tree) Find(fpath string) (*Match, bool) {
	fpath = path.Clean(fpath)
	if fpath == "." {
		return &Match{
			Path:       ".",
			Mode:       t.root.Mode,
			generators: t.root.Generators,
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
	}, true
}

type Match struct {
	Path       string
	Mode       Mode
	generators []Generator
}

func (m *Match) Generate(cache Cache, target string) (*virt.File, error) {
	vfile := &virt.File{
		Path: m.Path,
		Mode: m.Mode.FileMode(),
	}
	des := map[string]fs.DirEntry{}
	for _, generator := range m.generators {
		file, err := generator.Generate(cache, target)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			continue
		}
		vfile.Data = file.Data
		for _, entry := range file.Entries {
			des[entry.Name()] = entry
		}
	}
	for _, entry := range des {
		vfile.Entries = append(vfile.Entries, entry)
	}
	sort.Slice(vfile.Entries, func(i, j int) bool {
		return vfile.Entries[i].Name() < vfile.Entries[j].Name()
	})
	return vfile, nil
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
	return fmt.Sprintf("%s mode=%s generators=%v", n.Name, n.Mode, len(n.Generators))
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
