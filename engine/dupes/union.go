package dupes

// union is a disjoint set over indexes.
type union struct{ parent []int }

func newUnion(n int) *union {
	u := &union{parent: make([]int, n)}
	for i := range u.parent {
		u.parent[i] = i
	}
	return u
}
