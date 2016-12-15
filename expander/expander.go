// Package expander provides a generic text expansion interface.
package expander

import (
	"fmt"
	"path"
	"sync"
	"time"
)

var (
	registry = make(map[string]NewExpander)
	mu       sync.Mutex
)

// Expander is a text expansion backend.
type Expander interface {
	// Load initializes the Expander from on-disk settings.
	Load() error
	// Groups returns all children *Group entries.
	Groups() []*Group
	// Group returns the child *Group of the given name.
	Group(string) *Group
	// SetGroup upserts a *Group.
	SetGroup(*Group)
	// Write saves the Expander to disk.
	Write() error
}

// Group is a Snippet container.
type Group struct {
	Name     string
	Snippets []*Snippet
	Groups   []*Group
	Parent   *Group
}

// Path returns the Group's path relative to the Expander base.
func (g Group) Path() string {
	if g.Parent == nil {
		return g.Name
	}
	return path.Join(g.Parent.Path(), g.Name)
}

// Merge recursively merges the given Group's children groups and snippets.
func (g *Group) Merge(other *Group) {
	g.MergeAll(other.Groups)
	g.mergeSnippets(other.Snippets)
}

// MergeAll recursively merges the given list of groups to the corresponding child Group entry,
// creates a new child Group entry if it does not already exist.
func (g *Group) MergeAll(groups []*Group) {
	for _, right := range groups {
		found := false
		for _, left := range g.Groups {
			if left.Name == right.Name {
				found = true
				left.Merge(right)
				break
			}
		}
		if !found {
			g.Groups = append(g.Groups, right)
		}
	}
}

// mergeSnippets upserts the given snippets.
func (g *Group) mergeSnippets(snippets []*Snippet) {
	for _, snippet := range snippets {
		found := false
		for i, s := range g.Snippets {
			if snippet.Name == s.Name {
				found = true
				g.Snippets[i] = snippet
				break
			}
		}
		if !found {
			g.Snippets = append(g.Snippets, snippet)
		}
	}
}

func (g Group) String() string {
	return g.Name
}

// Snippet is an individual text expansion snippet.
type Snippet struct {
	Name    string
	Abbr    string
	Text    string
	Parent  *Group
	ModTime time.Time
}

// Path returns the Snippet's path relative to the Expander base.
func (s Snippet) Path() string {
	return path.Join(s.Parent.Path(), s.Name)
}

func (s Snippet) String() string {
	return s.Name
}

// NewExpander is a function to create a new Expander.
type NewExpander func() Expander

// Register adds a NewExpander with the given name to the registry.
func Register(name string, fun NewExpander) error {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[name]; ok {
		return fmt.Errorf("%v already registered", name)
	}
	registry[name] = fun
	return nil
}

// New returns an Expander and error for a given name.
func New(name string) (Expander, error) {
	if fun, ok := registry[name]; ok {
		return fun(), nil
	}
	return nil, fmt.Errorf("invalid expander: %v", name)
}
