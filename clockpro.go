package clockpro

import "container/list"

type pageState int

const (
	stateCold pageState = iota
	stateHot
	stateColdResident
)

type page[K comparable, V any] struct {
	key    K
	value  V
	state  pageState
	ref    bool          // reference bit
	test   bool          // test bit for cold pages
	listID int           // which list this page belongs to (0=H, 1=R, 2=M)
	elem   *list.Element // pointer to element in container/list for O(1) removal
}

// circularList wraps a container/list.List and tracks a hand (current element)
// to support CLOCK-style scans. We keep a cached size to avoid O(n) Len calls
// on every mutation (Len is O(1) but we mirror previous struct semantics).
type circularList[K comparable, V any] struct {
	l    *list.List    // underlying list; elements hold *page[K,V]
	hand *list.Element // current CLOCK hand; nil means list is empty
	size int           // mirrored size for convenience
}

func newCircularList[K comparable, V any]() *circularList[K, V] {
	return &circularList[K, V]{l: list.New()}
}

// insert pushes page just after the hand (or to front if list empty).
func (cl *circularList[K, V]) insert(p *page[K, V]) {
	var elem *list.Element
	if cl.hand == nil {
		// Empty list – push and make hand point to it.
		elem = cl.l.PushFront(p)
		cl.hand = elem
	} else {
		// Insert after hand for similar behaviour to original code.
		next := cl.hand.Next()
		if next == nil {
			elem = cl.l.PushBack(p)
		} else {
			elem = cl.l.InsertBefore(p, next)
		}
	}
	p.elem = elem
	cl.size++
}

// remove removes page p from the list in O(1).
func (cl *circularList[K, V]) remove(p *page[K, V]) {
	if p.elem == nil {
		return
	}
	// Adjust hand if it points to the element being removed.
	if cl.hand == p.elem {
		cl.hand = cl.hand.Next()
		if cl.hand == nil {
			cl.hand = cl.l.Front()
		}
	}
	cl.l.Remove(p.elem)
	p.elem = nil
	cl.size--
	if cl.size == 0 {
		cl.hand = nil
	}
}

// moveHand advances the hand pointer to the next position (wraps around).
func (cl *circularList[K, V]) moveHand() {
	if cl.hand != nil {
		cl.hand = cl.hand.Next()
		if cl.hand == nil {
			cl.hand = cl.l.Front()
		}
	}
}

// head returns the *page[K,V] currently under the hand (nil if list empty).
func (cl *circularList[K, V]) head() *page[K, V] {
	if cl.hand == nil {
		return nil
	}
	return cl.hand.Value.(*page[K, V])
}

type clock[K comparable, V any] struct {
	hot      *circularList[K, V]
	cold     *circularList[K, V]
	meta     *circularList[K, V]
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
		hot:      newCircularList[K, V](),
		cold:     newCircularList[K, V](),
		meta:     newCircularList[K, V](),
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
		victim := c.cold.head()
		if victim == nil {
			break
		}
		c.cold.moveHand()
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
		victim := c.hot.head()
		if victim == nil {
			break
		}
		c.hot.moveHand()
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
		victim := c.meta.head()
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
