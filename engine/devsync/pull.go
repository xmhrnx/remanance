package devsync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// syncLoop scans the folders every rescan, or when woken, and takes
// other devices' changes.
func (n *Node) syncLoop(rescan time.Duration) {
	defer n.wg.Done()
	t := time.NewTicker(rescan)
	defer t.Stop()
	n.syncAll(true)
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-t.C:
			n.syncAll(true)
		case <-n.wake:
			n.syncAll(true)
		}
	}
}

func (n *Node) syncAll(scanFirst bool) {
	n.mu.Lock()
	folders := append([]Folder(nil), n.cfg.Folders...)
	n.mu.Unlock()
	for _, f := range folders {
		if n.ctx.Err() != nil || f.Paused {
			continue
		}
		if scanFirst {
			n.scanFolder(f)
		}
		// Taking changes can find more to take, as files land.
		for range 3 {
			if !n.pullFolder(f) {
				break
			}
		}
	}
}

// scanFolder finds this device's changes and tells the others.
func (n *Node) scanFolder(f Folder) {
	n.mu.Lock()
	st, ix := n.state[f.ID], n.idx[f.ID]
	if st == nil || ix == nil {
		n.mu.Unlock()
		return
	}
	st.scanning = true
	// The scan works on a copy, so others can read the index meanwhile.
	work := &index{Files: make(map[string]File, len(ix.Files))}
	for k, v := range ix.Files {
		work.Files[k] = v
	}
	n.mu.Unlock()
	n.changed()

	changed, err := scan(n.ctx, f.Path, n.id, work)

	n.mu.Lock()
	st.scanning = false
	if err != nil {
		st.err = err.Error()
		n.mu.Unlock()
		n.changed()
		return
	}
	st.err = ""
	if f.Mode == ReceiveOnly {
		// Changes made here aren't sent; the index keeps what came in,
		// so they're overwritten by the next change from elsewhere.
		changed = nil
	} else {
		// Files fetched while the scan ran are newer than what it saw.
		for _, c := range changed {
			if cur, ok := ix.Files[c.Name]; ok && cur.Version.compare(work.Files[c.Name].Version) == newer {
				continue
			}
			ix.Files[c.Name] = work.Files[c.Name]
		}
		for k, v := range work.Files {
			if _, ok := ix.Files[k]; !ok {
				ix.Files[k] = v
			} else if cur := ix.Files[k]; cur.Hash == v.Hash && cur.Version.compare(v.Version) == equal {
				ix.Files[k] = v // the same file, its new date
			}
		}
		if len(changed) > 0 {
			saveJSON(n.indexPath(f.ID), ix)
		}
	}
	n.mu.Unlock()
	n.announce(f.ID, changed)
	n.changed()
}

// job is one file to take from another device.
type job struct {
	remote File
	from   *conn
	local  File
	have   bool
}

// pullFolder takes the changes the other devices have that this one
// doesn't, and tells whether it took any.
func (n *Node) pullFolder(f Folder) bool {
	if f.Mode == SendOnly {
		n.markSynced(f.ID)
		return false
	}
	jobs := n.plan(f)
	if len(jobs) == 0 {
		n.markSynced(f.ID)
		return false
	}
	n.mu.Lock()
	n.state[f.ID].pulling = true
	n.mu.Unlock()
	n.changed()
	var done []File
	var failed error
	for _, j := range jobs {
		if n.ctx.Err() != nil {
			break
		}
		file, err := n.take(f, j)
		if err != nil {
			failed = fmt.Errorf("%s: %w", j.remote.Name, err)
			continue
		}
		done = append(done, file)
	}
	n.mu.Lock()
	st := n.state[f.ID]
	if st != nil {
		st.pulling = false
		if failed != nil {
			st.err = failed.Error()
		}
	}
	if ix := n.idx[f.ID]; ix != nil && len(done) > 0 {
		saveJSON(n.indexPath(f.ID), ix)
	}
	n.mu.Unlock()
	n.announce(f.ID, done)
	n.changed()
	return len(done) > 0
}

func (n *Node) markSynced(folder string) {
	n.mu.Lock()
	if st := n.state[folder]; st != nil && st.err == "" {
		st.lastSync = time.Now()
	}
	n.mu.Unlock()
}

// plan lists what to take: for each file, the newest version any
// connected device has, when it's newer than this device's or a
// conflict this device loses.
func (n *Node) plan(f Folder) []job {
	n.mu.Lock()
	defer n.mu.Unlock()
	ix := n.idx[f.ID]
	var jobs []job
	best := map[string]job{}
	for dev, files := range n.remote[f.ID] {
		c := n.conns[dev]
		if c == nil || n.shared(f.ID, c) == nil {
			continue
		}
		for name, rf := range files {
			b, ok := best[name]
			if !ok || rf.Version.compare(b.remote.Version) == newer ||
				(rf.Version.compare(b.remote.Version) == concurrent && wins(rf, b.remote)) {
				best[name] = job{remote: rf, from: c}
			}
		}
	}
	for name, j := range best {
		local, have := ix.Files[name]
		j.local, j.have = local, have
		if !have {
			if j.remote.Deleted {
				continue // never here
			}
			jobs = append(jobs, j)
			continue
		}
		switch j.remote.Version.compare(local.Version) {
		case newer:
			jobs = append(jobs, j)
		case concurrent:
			if f.Mode == ReceiveOnly || wins(j.remote, local) {
				jobs = append(jobs, j)
			}
		}
	}
	// Small files first, so most of the folder is there soonest.
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].remote.Size < jobs[b].remote.Size })
	return jobs
}

// wins tells whether a's change beats b's when both changed a file: a
// deletion loses to an edit, then the later change wins, then the
// larger hash, so every device picks the same one.
func wins(a, b File) bool {
	if a.Deleted != b.Deleted {
		return !a.Deleted
	}
	if a.Mod != b.Mod {
		return a.Mod > b.Mod
	}
	return a.Hash > b.Hash
}

// take brings a file to the version another device has, and returns
// the index's new entry.
func (n *Node) take(f Folder, j job) (File, error) {
	dst := filepath.Join(f.Path, filepath.FromSlash(j.remote.Name))
	merged := j.remote.Version.merge(j.local.Version)
	entry := j.remote
	entry.Version = merged

	// A change here the last scan hasn't seen waits for the next.
	if j.have && !j.local.Deleted {
		fi, err := os.Stat(dst)
		if err != nil || fi.Size() != j.local.Size || fi.ModTime().UnixNano() != j.local.Mod {
			if !(err != nil && errors.Is(err, os.ErrNotExist) && j.remote.Deleted) {
				n.SyncNow()
				return File{}, errors.New("changed here meanwhile; trying again after the next scan")
			}
		}
	}
	concurrentEdit := j.have && j.remote.Version.compare(j.local.Version) == concurrent

	switch {
	case j.remote.Deleted:
		if j.have && !j.local.Deleted {
			if concurrentEdit && f.Mode != ReceiveOnly {
				// An edit here beats a deletion there: this is the
				// losing side only if the edit lost, which plan rules out.
				return File{}, errors.New("conflict")
			}
			if err := n.retire(f, dst, j.remote.Name); err != nil {
				return File{}, err
			}
		}
		entry.Size, entry.Hash = 0, ""
	case j.have && !j.local.Deleted && j.local.Hash == j.remote.Hash:
		// Already the same contents: only the version catches up.
		entry.Mod = j.local.Mod
	default:
		if concurrentEdit && !j.local.Deleted {
			// The change here lost: keep it beside the winner.
			n.mu.Lock()
			name := n.cfg.Name
			n.mu.Unlock()
			if err := os.Rename(dst, conflictName(dst, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return File{}, err
			}
		}
		if err := n.download(f, j, dst); err != nil {
			return File{}, err
		}
		fi, err := os.Stat(dst)
		if err != nil {
			return File{}, err
		}
		entry.Mod = fi.ModTime().UnixNano()
	}
	n.mu.Lock()
	if ix := n.idx[f.ID]; ix != nil {
		ix.Files[j.remote.Name] = entry
	}
	n.mu.Unlock()
	return entry, nil
}

// download fetches a file into a temporary file beside it, checks it
// against its hash and moves it into place, keeping what it replaces
// if the folder keeps versions.
func (n *Node) download(f Folder, j job, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(dst), ".datadoc-tmp-"+filepath.Base(dst))
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	for off := int64(0); off < j.remote.Size; {
		if n.ctx.Err() != nil {
			out.Close()
			os.Remove(tmp)
			return n.ctx.Err()
		}
		size := int(min(int64(blockSize), j.remote.Size-off))
		data, err := j.from.fetch(f.ID, j.remote.Name, j.remote.Hash, off, size)
		if err == nil && len(data) != size {
			err = errors.New("the other device sent less than asked")
		}
		if err != nil {
			out.Close()
			os.Remove(tmp)
			return err
		}
		h.Write(data)
		if _, err := out.Write(data); err != nil {
			out.Close()
			os.Remove(tmp)
			return err
		}
		off += int64(size)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != j.remote.Hash {
		os.Remove(tmp)
		return errors.New("the file arrived damaged; it will be fetched again")
	}
	mod := time.Unix(0, j.remote.Mod)
	os.Chtimes(tmp, mod, mod)
	if f.Versions {
		if _, err := os.Stat(dst); err == nil {
			if err := n.keepVersion(f, dst, j.remote.Name); err != nil {
				os.Remove(tmp)
				return err
			}
		}
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// retire takes away a file deleted on another device: into the
// versions folder if the folder keeps them.
func (n *Node) retire(f Folder, p, name string) error {
	if f.Versions {
		return n.keepVersion(f, p, name)
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// versionsDir is where a folder keeps replaced and deleted files.
const versionsDir = ".datadoc-versions"

func (n *Node) keepVersion(f Folder, p, name string) error {
	ext := path.Ext(name)
	kept := filepath.Join(f.Path, versionsDir, filepath.FromSlash(strings.TrimSuffix(name, ext)+"~"+time.Now().Format("20060102-150405")+ext))
	if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
		return err
	}
	if err := os.Rename(p, kept); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// conflictName is the name a losing change is kept under.
func conflictName(p, device string) string {
	ext := filepath.Ext(p)
	dev := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?* `, r) {
			return '-'
		}
		return r
	}, device)
	return strings.TrimSuffix(p, ext) + ".sync-conflict-" + time.Now().Format("20060102-150405") + "-" + dev + ext
}
