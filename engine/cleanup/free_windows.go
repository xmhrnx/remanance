package cleanup

import "golang.org/x/sys/windows"

func free(path string) (Space, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Space{}, err
	}
	var avail, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &totalFree); err != nil {
		return Space{}, err
	}
	return Space{Free: int64(avail), Total: int64(total)}, nil
}

// mine is true: what Windows lets the user see in their temporary
// folders, they may delete.
func mine(string) bool { return true }
