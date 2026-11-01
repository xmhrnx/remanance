package model

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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

func sortFiles(files []string, o RenameOrder) {
	switch o {
	case OrderByModified:
		mod := map[string]int64{}
		for _, f := range files {
			if fi, err := os.Stat(f); err == nil {
				mod[f] = fi.ModTime().UnixNano()
			}
		}
		sort.SliceStable(files, func(i, j int) bool { return mod[files[i]] < mod[files[j]] })
	default:
		sort.Slice(files, func(i, j int) bool { return naturalLess(files[i], files[j]) })
	}
}

// naturalLess orders names as people do: "IMG 2" before "IMG 10".
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digits(s string) int {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
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

// Replan works out the new names of the files included.
func (m *Model) Replan() {
	r := &m.Rename
	var paths []string
	for _, f := range r.Files {
		if !r.Skip[f] {
			paths = append(paths, f)
		}
	}
	r.Plan, r.PlanErr = rename.Plan(paths, r.Rules)
}

// RenameCounts counts the files that change and those with problems.
func (m *Model) RenameCounts() (changes, problems int) {
	for _, c := range m.Rename.Plan {
		switch {
		case c.Problem != "":
			problems++
		case !c.Same():
			changes++
		}
	}
	return changes, problems
}

// ApplyRename renames the files.
func (m *Model) ApplyRename() {
	r := &m.Rename
	done, err := rename.Apply(r.Plan)
	r.Err, r.Done = err, ""
	if err != nil {
		m.ListRenameFiles()
		return
	}
	r.Undo = done
	r.Done = fmt.Sprintf("Renamed %d %s", len(done), plural(len(done), "file", "files"))
	m.ListRenameFiles()
}

// UndoRename puts back the names the last rename changed.
func (m *Model) UndoRename() {
	r := &m.Rename
	if len(r.Undo) == 0 {
		return
	}
	n := len(r.Undo)
	r.Err = rename.Undo(r.Undo)
	if r.Err == nil {
		r.Done = fmt.Sprintf("Put back %d %s", n, plural(n, "name", "names"))
		r.Undo = nil
	}
	m.ListRenameFiles()
}

// CloseRename goes back to choosing a folder, keeping the rules.
func (m *Model) CloseRename() {
	rules := m.Rename.Rules
	m.Rename = RenameState{Rules: rules}
}
