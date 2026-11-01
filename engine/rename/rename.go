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
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
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

// CaseLabels name the changes of case, in order.
var CaseLabels = []string{"Keep case", "lowercase", "UPPERCASE", "Title Case", "Sentence case"}

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

// expand fills in a pattern's fields.
func expand(pattern, p, stem, ext string, n int) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(pattern, '{')
		if i < 0 {
			b.WriteString(pattern)
			return b.String()
		}
		j := strings.IndexByte(pattern[i:], '}')
		if j < 0 {
			b.WriteString(pattern)
			return b.String()
		}
		b.WriteString(pattern[:i])
		field, arg, _ := strings.Cut(pattern[i+1:i+j], ":")
		switch field {
		case "name":
			b.WriteString(stem)
		case "ext":
			b.WriteString(ext)
		case "n":
			s := strconv.Itoa(n)
			if w, err := strconv.Atoi(arg); err == nil && len(s) < w {
				s = strings.Repeat("0", w-len(s)) + s
			}
			b.WriteString(s)
		case "date":
			if arg == "" {
				arg = "2006-01-02"
			}
			b.WriteString(dateOf(p).Format(arg))
		case "parent":
			b.WriteString(filepath.Base(filepath.Dir(p)))
		default:
			b.WriteString(pattern[i : i+j+1])
		}
		pattern = pattern[i+j+1:]
	}
}

// dateOf is when a photo was taken, or when the file last changed.
func dateOf(p string) time.Time {
	if t, ok := photoDate(p); ok {
		return t
	}
	if info, err := os.Stat(p); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func changeCase(s string, c Case) string {
	switch c {
	case CaseLower:
		return strings.ToLower(s)
	case CaseUpper:
		return strings.ToUpper(s)
	case CaseTitle, CaseSentence:
		rs := []rune(strings.ToLower(s))
		start := true
		for i, r := range rs {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if start {
					rs[i] = unicode.ToUpper(r)
					start = false
				}
				continue
			}
			if c == CaseTitle && (r == ' ' || r == '_' || r == '-' || r == '.') {
				start = true
			}
		}
		return string(rs)
	}
	return s
}

// foldNames is true where the file system ignores case in names.
var foldNames = runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios"

func key(dir, name string) string {
	k := filepath.Join(dir, name)
	if foldNames {
		k = strings.ToLower(k)
	}
	return k
}

// check finds what's wrong with each new name.
func check(cs []Change) {
	moving := map[string]bool{} // files this batch renames
	for _, c := range cs {
		if !c.Same() {
			moving[key(filepath.Dir(c.Path), filepath.Base(c.Path))] = true
		}
	}
	taken := map[string]int{}
	for _, c := range cs {
		taken[key(filepath.Dir(c.Path), c.New)]++
	}
	for i := range cs {
		c := &cs[i]
		dir := filepath.Dir(c.Path)
		k := key(dir, c.New)
		switch {
		case c.New == "" || strings.HasPrefix(c.New, "."):
			c.Problem = "The name would be empty"
		case strings.ContainsAny(c.New, `/\`) || strings.ContainsAny(c.New, invalid) || hasControl(c.New):
			c.Problem = "The name has characters files can't have"
		case reserved(c.New):
			c.Problem = "The system reserves that name"
		case len(c.New) > 255:
			c.Problem = "The name is too long"
		case taken[k] > 1:
			c.Problem = "Another file would get the same name"
		case c.Same():
		case !moving[k] && exists(dir, c.New) && !strings.EqualFold(filepath.Base(c.Path), c.New):
			c.Problem = "A file with that name is already there"
		}
	}
}

func exists(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, name))
	return err == nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 32 {
			return true
		}
	}
	return false
}

// invalid are characters Windows doesn't allow in names, refused on
// every system so files can move between them.
const invalid = `<>:"|?*`

// reserved tells whether Windows reserves a name, such as CON or LPT1.
func reserved(name string) bool {
	stem := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
		return true
	}
	return strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ")
}

// Done is a rename that happened, for undoing it.
type Done struct {
	From, To string
}

// ErrProblems means some changes have problems; nothing was renamed.
var ErrProblems = errors.New("fix the names marked first")

// Apply renames the files. Each goes to a temporary name first and then
// to its new one, so two files can trade names. If a rename fails, the
// ones done so far are undone, and the error says which file failed.
func Apply(cs []Change) ([]Done, error) {
	for _, c := range cs {
		if c.Problem != "" {
			return nil, ErrProblems
		}
	}
	type step struct{ from, tmp, to string }
	var steps []step
	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	for i, c := range cs {
		if c.Same() {
			continue
		}
		dir := filepath.Dir(c.Path)
		steps = append(steps, step{c.Path, filepath.Join(dir, fmt.Sprintf(".rename-%s-%d", stamp, i)), filepath.Join(dir, c.New)})
	}
	var done []Done
	undoTmp := func(k int) {
		for j := k - 1; j >= 0; j-- {
			os.Rename(steps[j].tmp, steps[j].from)
		}
	}
	for k, s := range steps {
		if err := os.Rename(s.from, s.tmp); err != nil {
			undoTmp(k)
			return nil, fmt.Errorf("renaming %s: %w", filepath.Base(s.from), err)
		}
	}
	for k, s := range steps {
		if err := os.Rename(s.tmp, s.to); err != nil {
			Undo(done)
			// Those not yet moved go back from their temporary names.
			for _, r := range steps[k:] {
				os.Rename(r.tmp, r.from)
			}
			return nil, fmt.Errorf("renaming %s: %w", filepath.Base(s.from), err)
		}
		done = append(done, Done{From: s.from, To: s.to})
	}
	return done, nil
}

// Undo puts back the names Apply changed, in two steps like Apply, and
// returns what failed.
func Undo(done []Done) error {
	cs := make([]Change, 0, len(done))
	for _, d := range done {
		cs = append(cs, Change{Path: d.To, New: filepath.Base(d.From)})
	}
	_, err := Apply(cs)
	return err
}
