package components

import (
	"github.com/mehran9675/giode"
	"github.com/mehran9675/remanence/engine/fsys"
)

// StateColor is the color that marks a file's state.
func StateColor(s fsys.State) string {
	switch s {
	case fsys.Deleted:
		return "theme.primary"
	case fsys.Orphan:
		return "theme.accent"
	case fsys.Carved:
		return "theme.info"
	}
	return "theme.muted"
}

// StateLabel names a file's state for people.
func StateLabel(s fsys.State) string {
	switch s {
	case fsys.Deleted:
		return "Deleted"
	case fsys.Orphan:
		return "Orphaned"
	case fsys.Carved:
		return "By signature"
	}
	return "Existing"
}

// StatePill tags a file with its state.
func StatePill(s fsys.State) giode.Element { return Tag(StateLabel(s), StateColor(s)) }
