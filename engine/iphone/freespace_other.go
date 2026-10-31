//go:build !windows && !linux && !darwin

package iphone

// freeSpace isn't known here.
func freeSpace(string) int64 { return 0 }
