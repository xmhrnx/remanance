// Package rename renames many files at once by rules: a pattern built
// from each file's name, number, date and folder, find and replace
// (plain or with regular expressions), and changes of case. Plan shows
// every new name and what's wrong with it before anything changes;
// Apply renames in two steps, so names can swap, and returns what undoes
// it.
package rename

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Case is a change of case.
type Case int

const (
	CaseKeep     Case = iota
	CaseLower         // all lowercase
	CaseUpper         // ALL UPPERCASE
	CaseTitle         // Every Word Capitalized
	CaseSentence      // First word capitalized
)

// Rules say how to name the files. The pattern is applied first, then
// find and replace, then the case, to the name without its extension
// unless WithExt is set.
type Rules struct {
	// Pattern builds each name, "{name}" by default. It may hold:
	//   {name}    the name without its extension
	//   {ext}     the extension, without the dot
	//   {n}       a number counting up from Start by Step; {n:3} pads it
	//             to 3 digits
	//   {date}    when the photo was taken, from its EXIF data, or else
	//             when the file last changed: {date:2006-01-02 15.04}
	//             takes a Go time layout, "2006-01-02" by default
	//   {parent}  the name of the folder it's in
	Pattern     string
	Start, Step int

	Find, Replace string
	Regexp        bool // Find is a regular expression; Replace may use $1
	MatchCase     bool

	Case Case
	// Ext replaces the extension, without the dot; "" keeps it.
	Ext string
	// LowerExt lowercases the extension.
	LowerExt bool
	// WithExt applies find and replace and the case to the whole name.
	WithExt bool
}

// Change is one file's new name.
type Change struct {
	Path string // the file now
	New  string // its new name, in the same folder
	// Problem is why it can't be renamed; empty when it can.
	Problem string
}

// Done is a rename that happened, for undoing it.
type Done struct {
	From, To string
}

// CaseLabels name the changes of case, in order.
var CaseLabels = []string{"Keep case", "lowercase", "UPPERCASE", "Title Case", "Sentence case"}

// ErrProblems means some changes have problems; nothing was renamed.
var ErrProblems = errors.New("fix the names marked first")

// Same tells whether the name doesn't change.
func (c Change) Same() bool { return filepath.Base(c.Path) == c.New }

// Plan works out every file's new name, in the order given, and checks
// them: names that clash with each other or with files already there,
// names the system doesn't allow, and empty names.
func Plan(paths []string, r Rules) ([]Change, error) {
	var re *regexp.Regexp
	if r.Find != "" {
		expr := r.Find
		if !r.Regexp {
			expr = regexp.QuoteMeta(expr)
		}
		if !r.MatchCase {
			expr = "(?i)" + expr
		}
		var err error
		if re, err = regexp.Compile(expr); err != nil {
			return nil, fmt.Errorf("the search isn't a valid regular expression: %w", err)
		}
	}
	if r.Pattern == "" {
		r.Pattern = "{name}"
	}
	if r.Step == 0 {
		r.Step = 1
	}
	out := make([]Change, len(paths))
	for i, p := range paths {
		out[i] = Change{Path: p, New: newName(p, i, r, re)}
	}
	check(out)
	return out, nil
}

// newName names the i'th file.
func newName(p string, i int, r Rules, re *regexp.Regexp) string {
	base := filepath.Base(p)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	ext = strings.TrimPrefix(ext, ".")

	name := expand(r.Pattern, p, stem, ext, r.Start+i*r.Step)
	if r.WithExt && ext != "" && !strings.Contains(r.Pattern, "{ext}") {
		name += "." + ext
		ext = ""
	}
	if re != nil {
		repl := r.Replace
		if !r.Regexp {
			repl = strings.ReplaceAll(repl, "$", "$$")
		}
		name = re.ReplaceAllString(name, repl)
	}
	name = changeCase(name, r.Case)
	switch {
	case r.Ext != "":
		ext = strings.TrimPrefix(r.Ext, ".")
	case r.LowerExt:
		ext = strings.ToLower(ext)
	}
	name = strings.TrimSpace(name)
	if ext != "" && !strings.Contains(r.Pattern, "{ext}") {
		name += "." + ext
	}
	return name
}
