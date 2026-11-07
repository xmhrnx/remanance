package model

import (
	"github.com/xmhrnx/remanance/engine/rename"
	"github.com/xmhrnx/remanance/locales"
)

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
	r.Done = locales.N("rename.renamed", len(done))
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
		r.Done = locales.N("rename.put_back", n)
		r.Undo = nil
	}
	m.ListRenameFiles()
}
