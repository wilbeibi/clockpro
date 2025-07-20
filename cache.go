package clockpro

import "sync"

// Cache implements the CLOCK-Pro cache replacement algorithm using generics.
//
//	K must be comparable so it is valid as a map key.
//	V can be any type.
//
// All operations are safe for concurrent access.
type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	state *clockProState[K, V]
}

// New returns a new cache with the provided capacity. A non-positive size is
// clamped to 1.
func New[K comparable, V any](size int) *Cache[K, V] {
	return &Cache[K, V]{
		state: newClockProState[K, V](size),
	}
}

// Get retrieves a value from the cache and marks it as accessed
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p, exists := c.state.pageMap[key]
	if !exists {
		var zero V
		return zero, false
	}

	// Update access metadata based on current state
	switch p.state {
	case stateHot:
		p.ref = true

	case stateColdResident:
		if p.test {
			// Cold test page hit - promote to hot
			p.state = stateHot
			p.test = false
			p.ref = true
			c.state.coldList.remove(p)
			c.state.hotList.insert(p)
			p.listID = 0

			// Adjust capacity
			if c.state.hotList.size > c.state.hotCapacity {
				c.state.adaptHotCapacity(false)
			}
		} else {
			// Regular cold page hit
			p.ref = true
		}

	case stateCold:
		// Non-resident cold page hit
		if p.test {
			// This was a test page - adjust capacity and promote
			c.state.adaptHotCapacity(true)
		}

		// Remove from metadata list and make resident
		c.state.metaList.remove(p)

		// Make space if needed
		c.state.makeSpace()

		// Add as hot page
		p.state = stateHot
		p.test = false
		p.ref = true
		c.state.hotList.insert(p)
		p.listID = 0

		// Note: value was zero value for non-resident, caller needs to reload
		var zero V
		return zero, false
	}

	return p.value, true
}

// Put inserts or updates a key-value pair in the cache
func (c *Cache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if p, exists := c.state.pageMap[key]; exists {
		// Update existing entry
		p.value = value

		// Update access pattern as in Get
		switch p.state {
		case stateHot:
			p.ref = true

		case stateColdResident:
			if p.test {
				// Promote to hot
				p.state = stateHot
				p.test = false
				p.ref = true
				c.state.coldList.remove(p)
				c.state.hotList.insert(p)
				p.listID = 0

				if c.state.hotList.size > c.state.hotCapacity {
					c.state.adaptHotCapacity(false)
				}
			} else {
				p.ref = true
			}

		case stateCold:
			// Promote non-resident to hot
			if p.test {
				c.state.adaptHotCapacity(true)
			}

			c.state.metaList.remove(p)
			c.state.makeSpace()

			p.state = stateHot
			p.test = false
			p.ref = true
			c.state.hotList.insert(p)
			p.listID = 0
		}
		return
	}

	// New entry - make space first
	c.state.makeSpace()

	// Create new page and add to cold list initially
	newPage := &page[K, V]{
		key:    key,
		value:  value,
		state:  stateColdResident,
		ref:    false,
		test:   true, // new pages start as test pages
		listID: 1,
	}

	c.state.pageMap[key] = newPage
	c.state.coldList.insert(newPage)
}

// SetSize adjusts the total capacity of the cache
func (c *Cache[K, V]) SetSize(size int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if size <= 0 {
		size = 1
	}

	oldCapacity := c.state.capacity
	c.state.capacity = size

	// Adjust hot/cold split proportionally
	ratio := float64(c.state.hotCapacity) / float64(oldCapacity)
	newHotCapacity := int(float64(size) * ratio)
	if newHotCapacity == 0 {
		newHotCapacity = 1
	}
	if newHotCapacity >= size {
		newHotCapacity = size - 1
	}

	c.state.hotCapacity = newHotCapacity
	c.state.coldCapacity = size - newHotCapacity
	c.state.metaCapacity = size

	// Evict excess entries if capacity decreased
	for c.state.hotList.size+c.state.coldList.size > size {
		if c.state.coldList.size > 0 {
			c.state.evictColdPage()
		} else {
			c.state.evictHotPage()
		}
	}

	// Maintain metadata capacity
	c.state.maintainMetaCapacity()
}
