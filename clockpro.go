package clockpro

// pageState represents the state of a page in the CLOCK-Pro algorithm
type pageState int

const (
	stateCold pageState = iota
	stateHot
	stateColdResident
)

// page represents a single cache entry with metadata for CLOCK-Pro
type page[K comparable, V any] struct {
	key    K
	value  V
	state  pageState
	ref    bool        // reference bit
	test   bool        // test bit for cold pages
	next   *page[K, V] // next page in circular list
	prev   *page[K, V] // previous page in circular list
	listID int         // which list this page belongs to (0=H, 1=R, 2=M)
}

// circularList manages a doubly-linked circular list of pages
type circularList[K comparable, V any] struct {
	head *page[K, V]
	size int
}

// newCircularList creates a new empty circular list
func newCircularList[K comparable, V any]() *circularList[K, V] {
	return &circularList[K, V]{}
}

// insert adds a page to the list at the position after the hand
func (cl *circularList[K, V]) insert(p *page[K, V]) {
	if cl.head == nil {
		p.next = p
		p.prev = p
		cl.head = p
	} else {
		p.next = cl.head.next
		p.prev = cl.head
		cl.head.next.prev = p
		cl.head.next = p
	}
	cl.size++
}

// remove removes a page from the list
func (cl *circularList[K, V]) remove(p *page[K, V]) {
	if cl.size == 1 {
		cl.head = nil
	} else {
		if cl.head == p {
			cl.head = p.next
		}
		p.prev.next = p.next
		p.next.prev = p.prev
	}
	p.next = nil
	p.prev = nil
	cl.size--
}

// moveHand advances the hand pointer to the next position
func (cl *circularList[K, V]) moveHand() {
	if cl.head != nil {
		cl.head = cl.head.next
	}
}

// clockProState holds the internal state of the CLOCK-Pro algorithm
type clockProState[K comparable, V any] struct {
	hotList      *circularList[K, V] // H: hot pages list
	coldList     *circularList[K, V] // R: resident cold pages list
	metaList     *circularList[K, V] // M: non-resident cold pages list
	pageMap      map[K]*page[K, V]   // fast lookup for pages
	capacity     int                 // total cache capacity
	hotCapacity  int                 // maximum hot pages (adaptive)
	coldCapacity int                 // maximum cold pages
	metaCapacity int                 // maximum metadata entries
}

// newClockProState creates a new CLOCK-Pro state with the given capacity
func newClockProState[K comparable, V any](capacity int) *clockProState[K, V] {
	if capacity <= 0 {
		capacity = 1
	}

	// Initialize adaptive capacities based on the paper's recommendations
	hotCapacity := capacity / 2
	if hotCapacity == 0 {
		hotCapacity = 1
	}
	coldCapacity := capacity - hotCapacity
	metaCapacity := capacity // can be tuned based on memory constraints

	return &clockProState[K, V]{
		hotList:      newCircularList[K, V](),
		coldList:     newCircularList[K, V](),
		metaList:     newCircularList[K, V](),
		pageMap:      make(map[K]*page[K, V]),
		capacity:     capacity,
		hotCapacity:  hotCapacity,
		coldCapacity: coldCapacity,
		metaCapacity: metaCapacity,
	}
}

// evictColdPage removes the coldest page from the cold list
func (cp *clockProState[K, V]) evictColdPage() *page[K, V] {
	for cp.coldList.size > 0 {
		victim := cp.coldList.head
		cp.coldList.moveHand()

		if victim.ref {
			// Move to hot list
			victim.ref = false
			victim.state = stateHot
			cp.coldList.remove(victim)
			cp.hotList.insert(victim)
			victim.listID = 0

			// Continue searching for non-referenced page to evict
			continue
		} else {
			// Evict this cold page
			cp.coldList.remove(victim)
			delete(cp.pageMap, victim.key)

			// Add to metadata list if it was a test page
			if victim.test {
				var zero V
				victim.value = zero // remove actual data
				victim.state = stateCold
				victim.test = false
				cp.metaList.insert(victim)
				victim.listID = 2
				cp.maintainMetaCapacity()
			}

			return victim
		}
	}
	return nil
}

// evictHotPage removes a hot page and potentially converts it to cold
func (cp *clockProState[K, V]) evictHotPage() *page[K, V] {
	for cp.hotList.size > 0 {
		victim := cp.hotList.head
		cp.hotList.moveHand()

		if victim.ref {
			victim.ref = false
		} else {
			// Convert hot page to cold test page
			victim.state = stateColdResident
			victim.test = true
			victim.ref = false
			cp.hotList.remove(victim)
			cp.coldList.insert(victim)
			victim.listID = 1

			cp.adaptHotCapacity(true)
			return victim
		}
	}
	return nil
}

// adaptHotCapacity adjusts the hot capacity based on access patterns
func (cp *clockProState[K, V]) adaptHotCapacity(increase bool) {
	if increase && cp.hotCapacity < cp.capacity-1 {
		cp.hotCapacity++
		cp.coldCapacity--
	} else if !increase && cp.hotCapacity > 1 {
		cp.hotCapacity--
		cp.coldCapacity++
	}
}

// maintainMetaCapacity ensures metadata list doesn't exceed capacity
func (cp *clockProState[K, V]) maintainMetaCapacity() {
	for cp.metaList.size > cp.metaCapacity {
		victim := cp.metaList.head
		cp.metaList.remove(victim)
		delete(cp.pageMap, victim.key)
	}
}

// makeSpace creates space for a new entry by evicting if necessary
func (cp *clockProState[K, V]) makeSpace() {
	for cp.hotList.size+cp.coldList.size >= cp.capacity {
		// Need to evict - prefer cold pages first
		if cp.coldList.size > 0 {
			cp.evictColdPage()
		} else if cp.hotList.size > 0 {
			cp.evictHotPage()
		} else {
			break // No pages to evict
		}
	}
}
