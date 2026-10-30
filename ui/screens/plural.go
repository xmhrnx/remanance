package screens

import "fmt"

// count says n things, in the singular for one: "1 file", "3 files".
func count(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
