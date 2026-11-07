package screens

import (
	"fmt"

	"github.com/xmhrnx/remanance/locales"
)

// number writes a large count readably.
func number(n int64) string {
	switch {
	case n >= 1_000_000:
		return locales.T("space.million", "n", fmt.Sprintf("%.1f", float64(n)/1e6))
	case n >= 10_000:
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprint(n)
}
