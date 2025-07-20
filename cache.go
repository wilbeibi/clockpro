package clockpro

import "sync"

type Cache[K comparable, V any] struct {
	mu    sync.Mutex
	clock *clock[K, V]
}

func New[K comparable, V any](size int) *Cache[K, V] {
	return &Cache[K, V]{
		clock: newClock[K, V](size),
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, exists := c.clock.pageMap[key]
	if !exists {
		var zero V
		return zero, false
	}

	c.clock.touch(p)

	if p.state == stateCold {
		var zero V
		return zero, false
	}

	return p.value, true
}

func (c *Cache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if p, exists := c.clock.pageMap[key]; exists {
		p.value = value
		c.clock.touch(p)
		return
	}

	c.clock.makeSpace()

	newPage := &page[K, V]{
		key:   key,
		value: value,
		state: stateColdResident,
		test:  true,
	}

	c.clock.pageMap[key] = newPage
	c.clock.cold.insert(newPage)
}

func (c *Cache[K, V]) SetSize(size int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clock.resize(size)
}