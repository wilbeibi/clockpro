package clockpro

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		expected int
	}{
		{"normal size", 100, 100},
		{"zero size", 0, 1},
		{"negative size", -10, 1},
		{"small size", 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := New[interface{}, interface{}](tt.size)
			if cache == nil {
				t.Fatal("New() returned nil")
			}
			if cache.state.capacity != tt.expected {
				t.Errorf("New(%d) capacity = %d, want %d",
					tt.size, cache.state.capacity, tt.expected)
			}
		})
	}
}

func TestCacheBasicOperations(t *testing.T) {
	cache := New[interface{}, interface{}](3)

	// Test Get on empty cache
	if val, ok := cache.Get("key1"); ok {
		t.Errorf("Get on empty cache returned ok=true, val=%v", val)
	}

	// Test Put and Get
	cache.Put("key1", "value1")
	if val, ok := cache.Get("key1"); !ok || val != "value1" {
		t.Errorf("Get after Put: got (%v, %v), want (value1, true)", val, ok)
	}

	// Test update existing key
	cache.Put("key1", "newvalue1")
	if val, ok := cache.Get("key1"); !ok || val != "newvalue1" {
		t.Errorf("Get after update: got (%v, %v), want (newvalue1, true)", val, ok)
	}
}

func TestCacheEviction(t *testing.T) {
	cache := New[interface{}, interface{}](2) // Small cache to force eviction

	// Fill cache
	cache.Put("key1", "value1")
	cache.Put("key2", "value2")

	// Both should be present
	if _, ok := cache.Get("key1"); !ok {
		t.Error("key1 should be present")
	}
	if _, ok := cache.Get("key2"); !ok {
		t.Error("key2 should be present")
	}

	// Add third item to force eviction
	cache.Put("key3", "value3")

	// key3 should be present
	if _, ok := cache.Get("key3"); !ok {
		t.Error("key3 should be present after insertion")
	}

	// Check that total resident pages doesn't exceed capacity
	totalResident := cache.state.hotList.size + cache.state.coldList.size
	if totalResident > cache.state.capacity {
		t.Errorf("Total resident pages %d exceeds capacity %d",
			totalResident, cache.state.capacity)
	}
}

func TestHotColdTransitions(t *testing.T) {
	cache := New[interface{}, interface{}](4)

	// Add items as cold pages initially
	cache.Put("cold1", "value1")
	cache.Put("cold2", "value2")

	// Access cold pages multiple times to potentially promote to hot
	for i := 0; i < 3; i++ {
		cache.Get("cold1")
		cache.Get("cold2")
	}

	// Verify internal state consistency
	totalPages := len(cache.state.pageMap)
	listSizes := cache.state.hotList.size + cache.state.coldList.size + cache.state.metaList.size

	if totalPages != listSizes {
		t.Errorf("Page map size %d doesn't match list sizes %d", totalPages, listSizes)
	}

	// Check capacity constraints
	if cache.state.hotList.size > cache.state.hotCapacity {
		t.Errorf("Hot list size %d exceeds hot capacity %d",
			cache.state.hotList.size, cache.state.hotCapacity)
	}

	residentPages := cache.state.hotList.size + cache.state.coldList.size
	if residentPages > cache.state.capacity {
		t.Errorf("Resident pages %d exceed total capacity %d",
			residentPages, cache.state.capacity)
	}
}

func TestSetSize(t *testing.T) {
	cache := New[interface{}, interface{}](5)

	// Fill cache
	for i := 0; i < 5; i++ {
		cache.Put(i, i*10)
	}

	originalTotal := len(cache.state.pageMap)

	// Increase size
	cache.SetSize(10)
	if cache.state.capacity != 10 {
		t.Errorf("SetSize(10): capacity = %d, want 10", cache.state.capacity)
	}

	// Decrease size significantly to force evictions
	cache.SetSize(2)
	if cache.state.capacity != 2 {
		t.Errorf("SetSize(2): capacity = %d, want 2", cache.state.capacity)
	}

	// Check that evictions occurred
	residentPages := cache.state.hotList.size + cache.state.coldList.size
	if residentPages > 2 {
		t.Errorf("After SetSize(2), resident pages = %d, want <= 2", residentPages)
	}

	// Verify we can still add items
	cache.Put("new", "value")
	if _, ok := cache.Get("new"); !ok {
		t.Error("Should be able to add items after SetSize")
	}

	_ = originalTotal // Suppress unused variable warning
}

func TestAdaptiveCapacity(t *testing.T) {
	cache := New[interface{}, interface{}](10)

	// Record initial hot capacity
	initialHotCap := cache.state.hotCapacity

	// Add many items and access them to trigger adaptation
	for i := 0; i < 20; i++ {
		cache.Put(i, i)
		// Access some items multiple times
		if i < 10 {
			for j := 0; j < 3; j++ {
				cache.Get(i)
			}
		}
	}

	// Capacity should still be within bounds
	if cache.state.hotCapacity < 1 {
		t.Error("Hot capacity should be at least 1")
	}

	if cache.state.hotCapacity >= cache.state.capacity {
		t.Errorf("Hot capacity %d should be less than total capacity %d",
			cache.state.hotCapacity, cache.state.capacity)
	}

	if cache.state.hotCapacity+cache.state.coldCapacity != cache.state.capacity {
		t.Errorf("Hot capacity %d + cold capacity %d should equal total capacity %d",
			cache.state.hotCapacity, cache.state.coldCapacity, cache.state.capacity)
	}

	_ = initialHotCap // Suppress unused variable warning
}

func TestConcurrency(t *testing.T) {
	cache := New[interface{}, interface{}](100)

	// Test basic concurrent access
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			cache.Put(i, i*2)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 50; i++ {
			cache.Get(i)
		}
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done

	// Verify cache is still in consistent state
	totalResident := cache.state.hotList.size + cache.state.coldList.size
	if totalResident > cache.state.capacity {
		t.Errorf("After concurrent access, resident pages %d exceed capacity %d",
			totalResident, cache.state.capacity)
	}
}

func TestStateTransitions(t *testing.T) {
	cache := New[interface{}, interface{}](4)

	// Add a page and track its state transitions
	cache.Put("test", "value")

	page := cache.state.pageMap["test"]
	if page == nil {
		t.Fatal("Page should exist after Put")
	}

	// Initially should be cold resident with test bit
	if page.state != stateColdResident {
		t.Errorf("New page state = %v, want %v", page.state, stateColdResident)
	}

	if !page.test {
		t.Error("New page should have test bit set")
	}

	// Access it to potentially trigger promotion
	cache.Get("test")

	// Verify state is still valid
	if page.state != stateHot && page.state != stateColdResident {
		t.Errorf("Page state after access = %v, should be hot or cold resident", page.state)
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("capacity 1", func(t *testing.T) {
		cache := New[interface{}, interface{}](1)

		cache.Put("key1", "value1")
		if val, ok := cache.Get("key1"); !ok || val != "value1" {
			t.Errorf("Single item cache failed: got (%v, %v)", val, ok)
		}

		// Add second item, should evict first
		cache.Put("key2", "value2")
		if val, ok := cache.Get("key2"); !ok || val != "value2" {
			t.Errorf("After eviction: got (%v, %v), want (value2, true)", val, ok)
		}

		// Check total capacity is maintained
		total := cache.state.hotList.size + cache.state.coldList.size
		if total > 1 {
			t.Errorf("Capacity 1 cache has %d resident pages", total)
		}
	})

	t.Run("nil keys and values", func(t *testing.T) {
		cache := New[interface{}, interface{}](5)

		// nil key should work
		cache.Put(nil, "value")
		if val, ok := cache.Get(nil); !ok || val != "value" {
			t.Errorf("nil key failed: got (%v, %v)", val, ok)
		}

		// nil value should work
		cache.Put("key", nil)
		if val, ok := cache.Get("key"); !ok || val != nil {
			t.Errorf("nil value failed: got (%v, %v)", val, ok)
		}
	})
}
