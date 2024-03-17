package genfs

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/xlab/treeprint"
)

func newTree() *tree {
	return &tree{
		node: &node{
			children:   map[string]*node{},
			generators: nil,
			Path:       ".",
			Name:       ".",
			Mode:       modeDir,
			parent:     nil,
		},
	}
}

type tree struct {
	node *node
}

type node struct {
	Path string // path from root
	Name string // basename

	Mode       mode // mode of the file
	generators []generator

	children map[string]*node
	parent   *node
}

func (n *node) Children() (children []*node) {
	for _, child := range n.children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})
	return children
}

func shouldAppend(prevMode, nextMode mode) bool {
	return prevMode.IsGenDir() && nextMode.IsGenDir()
}

func shouldReplace(prevMode, nextMode mode) bool {
	return prevMode.IsDir() ||
		(prevMode.IsGenDir() && nextMode.IsGenFile())
}

func (t *tree) Insert(fpath string, mode mode, gen generator) {
	if fpath == "." {
		if shouldAppend(t.node.Mode, mode) {
			t.node.Mode = mode
			t.node.generators = append(t.node.generators, gen)
		} else if shouldReplace(t.node.Mode, mode) {
			t.node.Mode = mode
			t.node.generators = []generator{gen}
		}
		return
	}
	segments := strings.Split(fpath, "/")
	last := len(segments) - 1
	name := segments[last]
	parent := t.mkdirAll(segments[:last])
	// Add the base path with it's file generator to the tree.
	child, found := parent.children[name]
	if !found {
		child = &node{
			children:   map[string]*node{},
			generators: []generator{gen},
			Path:       fpath,
			Name:       name,
			Mode:       mode,
			parent:     parent,
		}
		// Add child to parent
		parent.children[name] = child
		return
	}
	// Create or update the child's attributes
	if shouldAppend(child.Mode, mode) {
		child.Mode = mode
		child.generators = append(child.generators, gen)
	} else if shouldReplace(child.Mode, mode) {
		child.Mode = mode
		child.generators = []generator{gen}
	}
}

func (t *tree) mkdirAll(segments []string) *node {
	parent := t.node
	for _, segment := range segments {
		child, ok := parent.children[segment]
		if !ok {
			child = &node{
				children:   map[string]*node{},
				generators: nil,
				Path:       path.Join(parent.Path, segment),
				Name:       segment,
				Mode:       modeDir,
				parent:     parent,
			}
			parent.children[segment] = child
		}
		parent = child
	}
	return parent
}

type match struct {
	Path       string
	generators []generator
	Mode       mode
	children   map[string]*node
}

func (m *match) Generate(cache Cache, target string) (fs.File, error) {
	for i := len(m.generators) - 1; i >= 0; i-- {
		if file, err := m.generators[i].Generate(cache, target); err == nil {
			return file, nil
		}
	}
	return nil, fs.ErrNotExist
}

type matchChild struct {
	Name string
	Path string
	Mode mode
}

func (m *match) Children() (children []*matchChild) {
	for name, child := range m.children {
		children = append(children, &matchChild{
			Name: name,
			Path: child.Path,
			Mode: child.Mode,
		})
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})
	return children
}

func (t *tree) find(path string) (n *node, ok bool) {
	// Special case to find the root node
	if path == "." {
		return t.node, true
	}
	// Traverse the children keyed by segments
	node := t.node
	segments := strings.Split(path, "/")
	for _, name := range segments {
		node, ok = node.children[name]
		if !ok {
			return nil, false
		}
	}
	return node, true
}

// Find an exact match the provided path
func (t *tree) Find(path string, accepts ...mode) (m *match, ok bool) {
	node, ok := t.find(path)
	if !ok {
		return nil, false
	}
	return &match{
		Path:       node.Path,
		generators: node.generators,
		Mode:       node.Mode,
		children:   node.children,
	}, true
}

func (t *tree) findPrefix(path string) (n *node, ok bool) {
	// Special case to find the root node
	if path == "." {
		return t.node, true
	}
	// Traverse the children keyed by segments
	node := t.node
	segments := strings.Split(path, "/")
	for _, name := range segments {
		child, ok := node.children[name]
		if !ok {
			// nodes that aren't dirs must be an exact match
			if !node.Mode.IsDir() {
				return nil, false
			}
			return node, true
		}
		node = child
	}
	return node, true
}

// Get the closest match to the provided path
func (t *tree) FindPrefix(path string, accepts ...mode) (m *match, ok bool) {
	node, ok := t.findPrefix(path)
	if !ok {
		return nil, false
	}
	return &match{
		Path:       node.Path,
		generators: node.generators,
		Mode:       node.Mode,
		children:   node.children,
	}, true
}

func (t *tree) Delete(paths ...string) {
	for _, path := range paths {
		if node, ok := t.find(path); ok {
			// We're trying to delete the root, ignore for now
			if node.parent == nil {
				continue
			}
			// Remove node from parent, deleting all descendants
			delete(node.parent.children, node.Name)
			node.parent = nil
		}
	}
}

func formatNode(node *node) string {
	if node.generators == nil {
		return fmt.Sprintf("%s mode=%s", node.Name, node.Mode)
	}
	gens := make([]string, len(node.generators))
	for i, gen := range node.generators {
		gens[i] = fmt.Sprintf("%s", gen)
	}
	return fmt.Sprintf("%s mode=%s generator=%v", node.Name, node.Mode, strings.Join(gens, ","))
}

func (t *tree) Print() string {
	tp := treeprint.NewWithRoot(formatNode(t.node))
	print(tp, t.node)
	return tp.String()
}

func print(tp treeprint.Tree, node *node) {
	for _, child := range node.Children() {
		cp := tp.AddBranch(formatNode(child))
		print(cp, child)
	}
}
