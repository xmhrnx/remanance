package disk

import (
	"container/list"
	"io"
	"sync"
)

// Cache keeps recently read blocks of a device in memory. File system
// parsers jump around metadata a few bytes at a time; without it every
// lookup is a device round trip, which is slow on USB and phones.
type Cache struct {
	r         io.ReaderAt
	size      int64
	blockSize int64
	max       int

	mu     sync.Mutex
	lru    *list.List
	blocks map[int64]*list.Element
}

type cacheBlock struct {
	index int64
	data  []byte
}

// NewCache caches r in blocks of blockSize bytes, holding at most
// maxBlocks of them.
func NewCache(r io.ReaderAt, size int64, blockSize, maxBlocks int) *Cache {
	return &Cache{r: r, size: size, blockSize: int64(blockSize), max: maxBlocks, lru: list.New(), blocks: map[int64]*list.Element{}}
}

func (c *Cache) block(i int64) ([]byte, error) {
	c.mu.Lock()
	if el, ok := c.blocks[i]; ok {
		c.lru.MoveToFront(el)
		b := el.Value.(*cacheBlock).data
		c.mu.Unlock()
		return b, nil
	}
	c.mu.Unlock()

	off := i * c.blockSize
	n := c.blockSize
	if off+n > c.size {
		n = c.size - off
	}
	data := make([]byte, n)
	if _, err := c.r.ReadAt(data, off); err != nil && err != io.EOF {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.blocks[i]; ok {
		return el.Value.(*cacheBlock).data, nil
	}
	c.blocks[i] = c.lru.PushFront(&cacheBlock{index: i, data: data})
	for c.lru.Len() > c.max {
		old := c.lru.Back()
		c.lru.Remove(old)
		delete(c.blocks, old.Value.(*cacheBlock).index)
	}
	return data, nil
}

// ReadAt reads through the cache. Reads larger than a block bypass it, so
// copying a big file doesn't evict the metadata the parsers need.
func (c *Cache) ReadAt(p []byte, off int64) (int, error) {
	if off >= c.size {
		return 0, io.EOF
	}
	if int64(len(p)) > c.blockSize*2 {
		return c.r.ReadAt(p, off)
	}
	done := 0
	for done < len(p) {
		pos := off + int64(done)
		if pos >= c.size {
			return done, io.EOF
		}
		b, err := c.block(pos / c.blockSize)
		if err != nil {
			return done, err
		}
		in := int(pos % c.blockSize)
		if in >= len(b) {
			return done, io.EOF
		}
		done += copy(p[done:], b[in:])
	}
	return done, nil
}
