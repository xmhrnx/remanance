package cleanup

import (
	"os"
	"path/filepath"
)

// Sources are the places Windows and its apps keep files that can go.
func Sources() []Source {
	home, _ := os.UserHomeDir()
	local := os.Getenv("LOCALAPPDATA")
	out := []Source{{Kind: Temp, Dir: os.TempDir(), Each: true}}
	if win := os.Getenv("SystemRoot"); win != "" {
		out = append(out, Source{Kind: Temp, Dir: filepath.Join(win, "Temp"), Each: true})
	}
	if local != "" {
		in := func(parts ...string) string { return filepath.Join(append([]string{local}, parts...)...) }
		out = append(out, places(Caches, "Windows web cache", in("Microsoft", "Windows", "INetCache"))...)
		out = append(out, places(Caches, "Graphics shader cache", in("D3DSCache"), in("NVIDIA", "DXCache"), in("NVIDIA", "GLCache"), in("AMD", "DxCache"))...)
		for _, b := range []struct{ name, dir string }{
			{"Google Chrome", in("Google", "Chrome", "User Data")},
			{"Microsoft Edge", in("Microsoft", "Edge", "User Data")},
			{"Brave", in("BraveSoftware", "Brave-Browser", "User Data")},
			{"Vivaldi", in("Vivaldi", "User Data")},
		} {
			out = append(out, places(Caches, b.name,
				filepath.Join(b.dir, "*", "Cache"), filepath.Join(b.dir, "*", "Code Cache"), filepath.Join(b.dir, "*", "GPUCache"))...)
		}
		out = append(out, places(Caches, "Firefox", in("Mozilla", "Firefox", "Profiles", "*", "cache2"))...)
		out = append(out, places(BuildCaches, "Go", in("go-build"))...)
		out = append(out, places(BuildCaches, "npm", in("npm-cache"))...)
		out = append(out, places(BuildCaches, "pip", in("pip", "cache"))...)
		out = append(out, places(BuildCaches, "Yarn", in("Yarn", "Cache"))...)
		out = append(out, Source{Kind: Logs, Dir: in("CrashDumps"), Each: true})
	}
	if data := os.Getenv("ProgramData"); data != "" {
		wer := filepath.Join(data, "Microsoft", "Windows", "WER")
		out = append(out, Source{Kind: Logs, Dir: filepath.Join(wer, "ReportArchive"), Each: true},
			Source{Kind: Logs, Dir: filepath.Join(wer, "ReportQueue"), Each: true})
	}
	out = append(out, Source{Kind: Trash})
	if home != "" {
		out = append(out, Source{Kind: OldDownloads, Dir: filepath.Join(home, "Downloads"), Each: true})
	}
	return out
}

// Drive is the drive Windows is on, whose space cleaning frees.
func Drive() string {
	if d := os.Getenv("SystemDrive"); d != "" {
		return d + `\`
	}
	return `C:\`
}
