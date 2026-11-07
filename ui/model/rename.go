package model

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/xmhrnx/remanance/engine/rename"
	"github.com/xmhrnx/remanance/ui/platform/androidstore"
)

// RenameOrder is the order files are numbered in.
type RenameOrder int

const (
	OrderByName RenameOrder = iota
	OrderByModified
)

// RenameState is the bulk renamer: the folder, its files, the rules and
// the names they give.
type RenameState struct {
	Dir       string
	Recursive bool
	Order     RenameOrder
	// Files are every file in the folder; Skip has those left alone.
	Files []string
	Skip  map[string]bool
	Rules rename.Rules
	Plan  []rename.Change
	// PlanErr is what's wrong with the rules, such as a bad regular
	// expression.
	PlanErr error
	// Undo undoes the last rename.
	Undo []rename.Done
	Err  error
	Done string
}

// OpenRenameFolder lists a folder's files to rename.
func (m *Model) OpenRenameFolder(dir string) {
	r := &m.Rename
	if err := androidstore.EnsureAccess(); err != nil {
		r.Err = err
		return
	}
	r.Dir, r.Err, r.Done, r.Undo = dir, nil, "", nil
	r.Skip = map[string]bool{}
	if r.Rules.Pattern == "" {
		r.Rules.Pattern = "{name}"
		r.Rules.Start, r.Rules.Step = 1, 1
	}
	m.ListRenameFiles()
}

// ListRenameFiles reads the folder again.
func (m *Model) ListRenameFiles() {
	r := &m.Rename
	var files []string
	walk := func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if p != r.Dir && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if p != r.Dir && !r.Recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			files = append(files, p)
		}
		return nil
	}
	if err := filepath.WalkDir(r.Dir, walk); err != nil {
		r.Err = err
	}
	sortFiles(files, r.Order)
	r.Files = files
	m.Replan()
}

// SetRenameRules changes the rules and works out the names again.
func (m *Model) SetRenameRules(rules rename.Rules) {
	m.Rename.Rules = rules
	m.Replan()
}

// ToggleRenameFile includes or leaves out a file.
func (m *Model) ToggleRenameFile(p string) {
	if m.Rename.Skip[p] {
		delete(m.Rename.Skip, p)
	} else {
		m.Rename.Skip[p] = true
	}
	m.Replan()
}

// CloseRename goes back to choosing a folder, keeping the rules.
func (m *Model) CloseRename() {
	rules := m.Rename.Rules
	m.Rename = RenameState{Rules: rules}
}
