package recovery

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/xmhrnx/remanance/engine/disk"
	"github.com/xmhrnx/remanance/engine/fsys"
)

// Item is a file or folder to recover, with the folder it goes under.
type Item struct {
	Entry *fsys.Entry
	// Base is the path the entry's own path is made relative to; files
	// under it keep their folders. "" places the entry at the top.
	Base string
	// Prefix goes in front of the destination path, such as the volume's
	// name when several are recovered at once.
	Prefix string
}

// Options control a recovery.
type Options struct {
	// KeepFolders recreates each file's folders; otherwise every file
	// goes straight into the destination.
	KeepFolders bool
	// Report writes "Recovery report.txt" listing what was copied.
	Report bool
}

// Progress reports how far a recovery is.
type Progress struct {
	Done, Total int64 // bytes
	Files       int   // files finished
	TotalFiles  int
	Current     string
}

// Outcome is what happened to one file.
type Outcome struct {
	Source string
	Dest   string
	Size   int64
	Health uint8
	Note   string
	Err    error
}

// Report sums up a recovery.
type Report struct {
	Location  string
	Recovered int
	Failed    int
	Bytes     int64
	Outcomes  []Outcome
	Duration  time.Duration
}

type job struct {
	e   *fsys.Entry
	rel string
}

// Recover copies items from dev into sink. Folders are recovered with
// everything under them.
func Recover(ctx context.Context, dev io.ReaderAt, items []Item, sink Sink, opts Options, progress func(Progress)) (*Report, error) {
	started := time.Now()
	var jobs []job
	seen := map[*fsys.Entry]bool{}
	for _, it := range items {
		it.Entry.Walk(func(e *fsys.Entry) bool {
			if e.IsDir || seen[e] {
				return true
			}
			seen[e] = true
			rel := SafeName(e.Name)
			if opts.KeepFolders {
				rel = relPath(e, it)
			}
			if it.Prefix != "" {
				rel = SafeName(it.Prefix) + "/" + rel
			}
			jobs = append(jobs, job{e: e, rel: rel})
			return true
		})
	}
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].rel < jobs[j].rel })

	var total int64
	for _, j := range jobs {
		total += j.e.Size
	}
	rep := &Report{Location: sink.Location()}
	p := Progress{Total: total, TotalFiles: len(jobs)}
	buf := make([]byte, 1<<20)
	lastSent := time.Time{}
	for _, j := range jobs {
		if err := ctx.Err(); err != nil {
			rep.Duration = time.Since(started)
			return rep, err
		}
		p.Current = j.e.Name
		rel := unique(sink, j.rel)
		out := Outcome{Source: j.e.Path(), Dest: rel, Size: j.e.Size, Health: j.e.Health, Note: j.e.Note}
		n, err := copyEntry(ctx, dev, j.e, sink, rel, buf, func(n int64) {
			p.Done += n
			if progress != nil && time.Since(lastSent) > 100*time.Millisecond {
				lastSent = time.Now()
				progress(p)
			}
		})
		out.Err = err
		if err != nil && n == 0 {
			// Nothing of it was read: leave no empty file behind.
			sink.Remove(rel)
			out.Dest = ""
		}
		if disk.IsGone(err) {
			// Every file after it would fail the same way.
			rep.Failed++
			rep.Outcomes = append(rep.Outcomes, out)
			rep.Duration = time.Since(started)
			if opts.Report {
				writeReport(sink, rep)
			}
			return rep, err
		}
		if err != nil {
			rep.Failed++
		} else {
			rep.Recovered++
			rep.Bytes += n
			sink.Finish(rel, j.e.Modified, j.e.Accessed)
		}
		rep.Outcomes = append(rep.Outcomes, out)
		p.Files++
	}
	if progress != nil {
		progress(p)
	}
	rep.Duration = time.Since(started)
	if opts.Report && len(jobs) > 0 {
		writeReport(sink, rep)
	}
	return rep, nil
}

// relPath keeps the entry's folders below the item's base.
func relPath(e *fsys.Entry, it Item) string {
	p := e.Path()
	if it.Base != "" && it.Base != "/" {
		p = strings.TrimPrefix(p, it.Base)
	}
	var parts []string
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s != "" {
			parts = append(parts, SafeName(s))
		}
	}
	if len(parts) == 0 {
		return SafeName(e.Name)
	}
	return path.Join(parts...)
}

func copyEntry(ctx context.Context, dev io.ReaderAt, e *fsys.Entry, sink Sink, rel string, buf []byte, step func(int64)) (int64, error) {
	r, err := fsys.Open(dev, e)
	if err != nil {
		return 0, err
	}
	w, err := sink.Create(rel)
	if err != nil {
		return 0, err
	}
	var done int64
	for done < e.Size {
		if ctx.Err() != nil {
			w.Close()
			return done, ctx.Err()
		}
		n := min(int64(len(buf)), e.Size-done)
		m, rerr := r.ReadAt(buf[:n], done)
		if m > 0 {
			if _, werr := w.Write(buf[:m]); werr != nil {
				w.Close()
				return done, werr
			}
			done += int64(m)
			step(int64(m))
		}
		if rerr != nil && rerr != io.EOF {
			w.Close()
			return done, rerr
		}
		if m == 0 {
			break
		}
	}
	return done, w.Close()
}

func writeReport(sink Sink, rep *Report) {
	w, err := sink.Create(unique(sink, "Recovery report.txt"))
	if err != nil {
		return
	}
	defer w.Close()
	fmt.Fprintf(w, "DataDoc recovery report\r\n%s\r\n\r\n", time.Now().Format("2 January 2006 15:04"))
	fmt.Fprintf(w, "Recovered %d files (%s), %d failed, in %s.\r\n\r\n", rep.Recovered, FormatBytes(rep.Bytes), rep.Failed, rep.Duration.Round(time.Second))
	for _, o := range rep.Outcomes {
		status := "OK"
		switch {
		case o.Err != nil:
			status = "FAILED: " + o.Err.Error()
		case o.Health < 100:
			status = fmt.Sprintf("recovered, %d%% intact", o.Health)
		}
		if o.Dest == "" {
			fmt.Fprintf(w, "%s (%s) %s\r\n", o.Source, FormatBytes(o.Size), status)
		} else {
			fmt.Fprintf(w, "%s\r\n    -> %s (%s) %s\r\n", o.Source, o.Dest, FormatBytes(o.Size), status)
		}
		if o.Note != "" {
			fmt.Fprintf(w, "    %s\r\n", o.Note)
		}
	}
}

// FormatBytes renders a size the way people read it.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
