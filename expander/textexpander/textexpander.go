// Package textexpander is an Expander for TextExpander.
package textexpander

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"time"

	plist "github.com/DHowett/go-plist"
	"github.com/hazaelsan/spandex/expander"
)

func init() {
	if err := expander.Register("TextExpander", newTE); err != nil {
		log.Fatal(err)
	}
}

// Command line flags.
var (
	teFile = flag.String("te_file", path.Join(os.Getenv("HOME"), "Dropbox/TextExpander/Settings.textexpander"), "TextExpander settings file")
)

// TextExpander is a plist-based Expander.
type TextExpander struct {
	file   string
	data   *rawData
	groups []*expander.Group
	mu     sync.RWMutex
}

func newTE() expander.Expander {
	return &TextExpander{
		file: *teFile,
	}
}

type rawData struct {
	Groups   []rawGroup   `plist:"groupsTE2"`
	Snippets []rawSnippet `plist:"snippetsTE2"`
}

type rawGroup struct {
	Name  string   `plist:"name"`
	UUIDs []string `plist:"snippetUUIDs"`
}

type rawSnippet struct {
	Abbr    string    `plist:"abbreviation"`
	Label   string    `plist:"label"`
	Text    string    `plist:"plainText"`
	UUID    string    `plist:"uuidString"`
	ModDate time.Time `plist:"modificationDate"`
}

// Load initializes all settings from disk.
func (te *TextExpander) Load() error {
	te.mu.Lock()
	defer te.mu.Unlock()
	f, err := os.Open(te.file)
	if err != nil {
		return err
	}
	defer f.Close()
	te.data = &rawData{}
	decoder := plist.NewDecoder(f)
	if err := decoder.Decode(te.data); err != nil {
		return err
	}
	return te.parse()
}

// Groups returns all children group entries.
func (te *TextExpander) Groups() []*expander.Group {
	te.mu.RLock()
	defer te.mu.RUnlock()
	return te.groups
}

// Group returns the child group of the given name.
func (te *TextExpander) Group(name string) *expander.Group {
	te.mu.RLock()
	defer te.mu.RUnlock()
	for _, group := range te.groups {
		if name == group.Name {
			return group
		}
	}
	return nil
}

// SetGroup upserts the given group.
func (te *TextExpander) SetGroup(group *expander.Group) {
	te.mu.Lock()
	defer te.mu.Unlock()
	for i, g := range te.groups {
		if group.Name == g.Name {
			te.groups[i] = group
			return
		}
	}
	te.groups = append(te.groups, group)
}

// Write is not implemented yet.
func (te *TextExpander) Write() error {
	return errors.New("not implemented")
}

// parse loads all groups and snippets from raw plist data.
func (te *TextExpander) parse() error {
	snippets := make(map[string]*expander.Snippet)
	for _, s := range te.data.Snippets {
		snippets[s.UUID] = &expander.Snippet{
			Name:    s.UUID,
			Abbr:    s.Abbr,
			Text:    s.Text,
			ModTime: s.ModDate,
		}
	}
	for _, g := range te.data.Groups {
		group := &expander.Group{Name: g.Name}
		for _, uuid := range g.UUIDs {
			s, ok := snippets[uuid]
			if !ok {
				return fmt.Errorf("invalid snippet UUID: %v", uuid)
			}
			s.Parent = group
			group.Snippets = append(group.Snippets, s)
		}
		te.groups = append(te.groups, group)
	}
	return nil
}
