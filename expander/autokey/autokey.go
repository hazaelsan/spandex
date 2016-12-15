// Package autokey is an Expander for AutoKey.
package autokey

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/hazaelsan/spandex/expander"
)

func init() {
	if err := expander.Register("AutoKey", newAK); err != nil {
		log.Fatal(err)
	}
}

const (
	dirMode     os.FileMode = 0755
	fileMode    os.FileMode = 0644
	dataDir                 = "data"
	metadataExt             = "json"
	snippetExt              = "txt"
)

// Command line flags.
var (
	akDir = flag.String("ak_dir", path.Join(os.Getenv("HOME"), ".config/autokey"), "AutoKey settings directory")
)

var filePattern = regexp.MustCompile(`^(.+/)?(.+)\.(.+?)$`)

type rawGroup struct {
	group   *expander.Group
	managed bool
}

// AutoKey is a JSON-based Expander.
type AutoKey struct {
	dir    string
	groups map[string]rawGroup
	mu     sync.RWMutex
}

func newAK() expander.Expander {
	return &AutoKey{dir: path.Join(*akDir, dataDir),
		groups: make(map[string]rawGroup),
	}
}

// Load initializes all settings from disk.
func (ak *AutoKey) Load() error {
	ak.mu.Lock()
	defer ak.mu.Unlock()
	root, err := ak.load(ak.dir, nil)
	if err != nil {
		return err
	}
	for _, g := range root.Groups {
		g.Parent = nil
		ak.groups[g.Name] = rawGroup{group: g}
	}
	return nil
}

// Groups returns all children group entries.
func (ak *AutoKey) Groups() []*expander.Group {
	ak.mu.RLock()
	defer ak.mu.RUnlock()
	var groups []*expander.Group
	for _, g := range ak.groups {
		groups = append(groups, g.group)
	}
	return groups
}

// Group returns the child group of the given name.
func (ak *AutoKey) Group(name string) *expander.Group {
	ak.mu.RLock()
	defer ak.mu.RUnlock()
	for _, g := range ak.groups {
		if name == g.group.Name {
			return g.group
		}
	}
	return nil
}

// SetGroup upserts the given group.
func (ak *AutoKey) SetGroup(group *expander.Group) {
	ak.mu.Lock()
	defer ak.mu.Unlock()
	g := rawGroup{
		group:   group,
		managed: true,
	}
	ak.groups[g.group.Name] = g
}

// Write recursively writes all children groups to disk.
func (ak *AutoKey) Write() error {
	ak.mu.Lock()
	defer ak.mu.Unlock()
	for _, g := range ak.groups {
		if !g.managed {
			continue
		}
		if err := ak.writeGroup(g.group); err != nil {
			return err
		}
	}
	return nil
}

// writeGroup recursively writes all children groups and snippets to disk.
func (ak *AutoKey) writeGroup(group *expander.Group) error {
	glog.Infof("Writing group %v", group)
	if err := os.MkdirAll(path.Join(ak.dir, group.Path()), dirMode); err != nil {
		return err
	}
	for _, g := range group.Groups {
		if err := ak.writeGroup(g); err != nil {
			return err
		}
	}
	for _, s := range group.Snippets {
		if err := ak.writeSnippet(s); err != nil {
			return err
		}
	}
	return nil
}

// writeSnippet writes the given Snippet to disk.
func (ak *AutoKey) writeSnippet(snippet *expander.Snippet) error {
	sPath := path.Join(ak.dir, fmt.Sprintf("%v.%v", snippet.Path(), snippetExt))
	mdPath, err := metadataPath(sPath)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(mdPath); err == nil {
		if !fi.ModTime().Before(snippet.ModTime) {
			glog.Infof("Snippet %v up to date", snippet)
			return nil
		}
	}
	glog.Infof("Writing snippet %v", snippet)
	if err := writeMetadata(mdPath, newMetadata(snippet)); err != nil {
		return err
	}
	if err := os.Chtimes(mdPath, snippet.ModTime, snippet.ModTime); err != nil {
		return err
	}
	if err := ioutil.WriteFile(sPath, []byte(snippet.Text), fileMode); err != nil {
		return err
	}
	return os.Chtimes(sPath, snippet.ModTime, snippet.ModTime)
}

// load recursively loads all groups and snippets from the given directory.
func (ak *AutoKey) load(dir string, parent *expander.Group) (*expander.Group, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	g := &expander.Group{
		Name:   filepath.Base(dir),
		Parent: parent,
	}
	for _, fi := range files {
		if fi.IsDir() {
			child, err := ak.load(path.Join(dir, fi.Name()), g)
			if err != nil {
				return nil, err
			}
			g.Groups = append(g.Groups, child)
		} else if fi.Mode().IsRegular() && !strings.HasPrefix(fi.Name(), ".") {
			s, err := newSnippet(path.Join(dir, fi.Name()))
			if err != nil {
				return nil, err
			}
			s.Parent = g
			g.Snippets = append(g.Snippets, s)
		}
	}
	return g, nil
}

type metadata struct {
	UsageCount     int            `json:"usageCount"`
	OmitTrigger    bool           `json:"omitTrigger"`
	Prompt         bool           `json:"prompt"`
	Description    string         `json:"description"`
	Abbreviation   mdAbbreviation `json:"abbreviation"`
	Hotkey         mdHotkey       `json:"hotkey"`
	Modes          []int          `json:"modes"`
	ShowInTrayMenu bool           `json:"showInTrayMenu"`
	MatchCase      bool           `json:"matchCase"`
	Filter         mdFilter       `json:"filter"`
	Type           string         `json:"type"`
	SendMode       string         `json:"sendMode"`
	ModTime        time.Time      `json:"-"`
}

type mdAbbreviation struct {
	WordChars     string   `json:"wordChars"`
	Abbreviations []string `json:"abbreviations"`
	Immediate     bool     `json:"immediate"`
	IgnoreCase    bool     `json:"ignoreCase"`
	Backspace     bool     `json:"backspace"`
	TriggerInside bool     `json:"triggerInside"`
}

type mdHotkey struct {
	HotKey    *string  `json:"hotKey"`
	Modifiers []string `json:"modifiers"`
}

type mdFilter struct {
	Regex       *string `json:"regex"`
	IsRecursive bool    `json:"isRecursive"`
}

// newSnippet returns a new *Snippet from the given file name.
func newSnippet(file string) (*expander.Snippet, error) {
	mdPath, err := metadataPath(file)
	if err != nil {
		return nil, err
	}
	md, err := parseMetadata(mdPath)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	s := &expander.Snippet{
		Name:    md.Description,
		Text:    string(buf),
		ModTime: md.ModTime,
	}
	if len(md.Abbreviation.Abbreviations) > 0 {
		s.Abbr = md.Abbreviation.Abbreviations[0]
	}
	return s, nil
}

// metadataPath returns the path to a snippet's metadata.
func metadataPath(file string) (string, error) {
	match := filePattern.FindStringSubmatch(file)
	if match == nil {
		return "", fmt.Errorf("invalid file name: %v", file)
	}
	return fmt.Sprintf("%v.%v.%v", match[1], match[2], metadataExt), nil
}

// newMetadata returns a new *metadata for the given Snippet.
func newMetadata(s *expander.Snippet) *metadata {
	md := &metadata{
		Description: s.Abbr,
		Abbreviation: mdAbbreviation{
			WordChars:     `[\w]`,
			Abbreviations: []string{},
			Immediate:     true,
			Backspace:     true,
		},
		Hotkey: mdHotkey{
			Modifiers: []string{},
		},
		Modes:    []int{1},
		Type:     "phrase",
		SendMode: "kb",
		ModTime:  s.ModTime,
	}
	if s.Abbr != "" {
		md.Abbreviation.Abbreviations = append(md.Abbreviation.Abbreviations, s.Abbr)
	}
	return md
}

// parseMetadata returns a new *metadata from the given file.
func parseMetadata(file string) (*metadata, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	md := &metadata{ModTime: fi.ModTime()}
	dec := json.NewDecoder(f)
	if err := dec.Decode(md); err != nil {
		return nil, err
	}
	return md, nil
}

// writeMetadata writes the given *metadata to the given file name.
func writeMetadata(file string, md *metadata) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(md)
}
