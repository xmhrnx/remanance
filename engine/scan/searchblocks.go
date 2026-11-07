package scan

type block struct {
	pos  int64
	data []byte
	bad  bool
}
