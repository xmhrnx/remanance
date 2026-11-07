package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xmhrnx/remanance/engine/rename"
)

// runRename renames files by a pattern, showing the new names first
// unless -yes is given.
func runRename(args []string) error {
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	var r rename.Rules
	fs.StringVar(&r.Pattern, "pattern", "{name}", "the new name: {name} {ext} {n:3} {date:2006-01-02} {parent}")
	fs.IntVar(&r.Start, "start", 1, "where {n} starts")
	fs.IntVar(&r.Step, "step", 1, "how much {n} counts up by")
	fs.StringVar(&r.Find, "find", "", "text to replace")
	fs.StringVar(&r.Replace, "replace", "", "what replaces it")
	fs.BoolVar(&r.Regexp, "regexp", false, "-find is a regular expression")
	fs.StringVar(&r.Ext, "ext", "", "a new extension")
	fs.BoolVar(&r.LowerExt, "lower-ext", false, "lowercase extensions")
	cs := fs.String("case", "", "lower, upper, title or sentence")
	yes := fs.Bool("yes", false, "rename, rather than only show the new names")
	fs.Parse(args)
	if fs.NArg() == 0 {
		return errors.New("usage: datadoc-cli rename -pattern P [-find X -replace Y] [-yes] FILE...")
	}
	switch strings.ToLower(*cs) {
	case "lower":
		r.Case = rename.CaseLower
	case "upper":
		r.Case = rename.CaseUpper
	case "title":
		r.Case = rename.CaseTitle
	case "sentence":
		r.Case = rename.CaseSentence
	}
	paths := fs.Args()
	sort.Strings(paths)
	plan, err := rename.Plan(paths, r)
	if err != nil {
		return err
	}
	for _, c := range plan {
		if c.Same() {
			continue
		}
		line := fmt.Sprintf("%s  ->  %s", filepath.Base(c.Path), c.New)
		if c.Problem != "" {
			line += "   (" + c.Problem + ")"
		}
		fmt.Println(line)
	}
	if !*yes {
		fmt.Println("\nNothing renamed yet: run again with -yes.")
		return nil
	}
	done, err := rename.Apply(plan)
	fmt.Printf("%d files renamed\n", len(done))
	return err
}
