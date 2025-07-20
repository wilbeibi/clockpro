package clockpro

type pageState int

const (
	stateCold pageState = iota
	stateHot
	stateColdResident
)

type page[K comparable, V any] struct {
	key   K
	value V
	state pageState
	ref   bool
	test  bool
	next  *page[K, V]
	prev  *page[K, V]
}

type ring[K comparable, V any] struct {
	head *page[K, V]
	size int
}

func newRing[K comparable, V any]() *ring[K, V] {
	return &ring[K, V]{}
}

func (r *ring[K, V]) insert(p *page[K, V]) {
	if r.size == 0 {
		p.next = p
		p.prev = p
		r.head = p
	} else {
		p.next = r.head.next
		p.prev = r.head
		r.head.next.prev = p
		r.head.next = p
	}
	r.size++
}

func (r *ring[K, V]) remove(p *page[K, V]) {
	if r.size == 1 {
		r.head = nil
	} else {
		if r.head == p {
			r.head = p.next
		}
		p.prev.next = p.next
		p.next.prev = p.prev
	}
	p.next = nil
	p.prev = nil
	r.size--
}

func (r *ring[K, V]) hand() *page[K, V] {
	return r.head
}

func (r *ring[K, V]) advance() {
	if r.head != nil {
		r.head = r.head.next
	}
}

type clock[K comparable, V any] struct {
	hot      *ring[K, V]
	cold     *ring[K, V]
	meta     *ring[K, V]
	pageMap  map[K]*page[K, V]
	capacity int
	hotCap   int
	coldCap  int
	metaCap  int
}

func newClock[K comparable, V any](capacity int) *clock[K, V] {
	if capacity <= 0 {
		capacity = 1
	}

	hotCap := capacity >> 1
	if hotCap == 0 {
		hotCap = 1
	}
	coldCap := capacity - hotCap

	return &clock[K, V]{
		hot:      newRing[K, V](),
		cold:     newRing[K, V](),
		meta:     newRing[K, V](),
		pageMap:  make(map[K]*page[K, V]),
		capacity: capacity,
		hotCap:   hotCap,
		coldCap:  coldCap,
		metaCap:  capacity,
	}
}

func (c *clock[K, V]) touch(p *page[K, V]) {
	switch p.state {
	case stateHot:
		p.ref = true

	case stateColdResident:
		if p.test {
			p.state = stateHot
			p.test = false
			p.ref = true
			c.cold.remove(p)
			c.hot.insert(p)

			if c.hot.size > c.hotCap {
				c.adaptHot(false)
			}
		} else {
			p.ref = true
		}

	case stateCold:
		if p.test {
			c.adaptHot(true)
		}

		c.meta.remove(p)
		c.makeSpace()

		p.state = stateHot
		p.test = false
		p.ref = true
		c.hot.insert(p)
	}
}

func (c *clock[K, V]) adaptHot(increase bool) {
	if increase && c.hotCap < c.capacity-1 {
		c.hotCap++
		c.coldCap--
	} else if !increase && c.hotCap > 1 {
		c.hotCap--
		c.coldCap++
	}
}

func (c *clock[K, V]) evictCold() *page[K, V] {
	steps := 0
	maxSteps := c.cold.size * 2

	for c.cold.size > 0 && steps < maxSteps {
		victim := c.cold.hand()
		if victim == nil {
			break
		}
		c.cold.advance()
		steps++

		if victim.ref {
			victim.ref = false
			victim.state = stateHot
			c.cold.remove(victim)
			c.hot.insert(victim)
			continue
		}

		c.cold.remove(victim)
		delete(c.pageMap, victim.key)

		if victim.test {
			var zero V
			victim.value = zero
			victim.state = stateCold
			victim.test = false
			c.meta.insert(victim)
			c.trimMeta()
		}

		return victim
	}
	return nil
}

func (c *clock[K, V]) evictHot() *page[K, V] {
	steps := 0
	maxSteps := c.hot.size * 2

	for c.hot.size > 0 && steps < maxSteps {
		victim := c.hot.hand()
		if victim == nil {
			break
		}
		c.hot.advance()
		steps++

		if victim.ref {
			victim.ref = false
		} else {
			c.hot.remove(victim)
			
			if c.cold.size < c.coldCap {
				victim.state = stateColdResident
				victim.test = true
				victim.ref = false
				c.cold.insert(victim)
				c.adaptHot(true)
			} else {
				delete(c.pageMap, victim.key)
			}
			return victim
		}
	}
	return nil
}

func (c *clock[K, V]) trimMeta() {
	for c.meta.size > c.metaCap {
		victim := c.meta.hand()
		if victim == nil {
			break
		}
		c.meta.remove(victim)
		delete(c.pageMap, victim.key)
	}
}

func (c *clock[K, V]) makeSpace() {
	for c.hot.size+c.cold.size >= c.capacity {
		if c.cold.size > 0 {
			if c.evictCold() == nil {
				break
			}
		} else if c.hot.size > 0 {
			if c.evictHot() == nil {
				break
			}
		} else {
			break
		}
	}
}

func (c *clock[K, V]) resize(size int) {
	if size <= 0 {
		size = 1
	}

	oldCap := c.capacity
	c.capacity = size

	ratio := (c.hotCap * 1000) / oldCap
	newHotCap := (size * ratio) / 1000
	if newHotCap == 0 {
		newHotCap = 1
	}
	if newHotCap >= size {
		newHotCap = size - 1
	}

	c.hotCap = newHotCap
	c.coldCap = size - newHotCap
	c.metaCap = size

	for c.hot.size+c.cold.size > size {
		evicted := false
		if c.cold.size > 0 {
			victim := c.evictCold()
			if victim != nil {
				evicted = true
			}
		}
		if !evicted && c.hot.size > 0 {
			victim := c.evictHot()
			if victim != nil {
				evicted = true
			}
		}
		if !evicted {
			break
		}
	}

	c.trimMeta()
}