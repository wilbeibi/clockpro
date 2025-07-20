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
			if cache.clock.capacity != tt.expected {
				t.Errorf("New(%d) capacity = %d, want %d",
					tt.size, cache.clock.capacity, tt.expected)
			}
		})
	}
}

func TestCacheBasicOperations(t *testing.T) {
	cache := New[interface{}, interface{}](3)

	if val, ok := cache.Get("key1"); ok {
		t.Errorf("Get on empty cache returned ok=true, val=%v", val)
	}

	cache.Put("key1", "value1")
	if val, ok := cache.Get("key1"); !ok || val != "value1" {
		t.Errorf("Get after Put: got (%v, %v), want (value1, true)", val, ok)
	}

	cache.Put("key1", "newvalue1")
	if val, ok := cache.Get("key1"); !ok || val != "newvalue1" {
		t.Errorf("Get after update: got (%v, %v), want (newvalue1, true)", val, ok)
	}
}

func TestCacheEviction(t *testing.T) {
	cache := New[interface{}, interface{}](2)

	cache.Put("key1", "value1")
	cache.Put("key2", "value2")

	if _, ok := cache.Get("key1"); !ok {
		t.Error("key1 should be present")
	}
	if _, ok := cache.Get("key2"); !ok {
		t.Error("key2 should be present")
	}

	cache.Put("key3", "value3")

	if _, ok := cache.Get("key3"); !ok {
		t.Error("key3 should be present after insertion")
	}

	totalResident := cache.clock.hot.size + cache.clock.cold.size
	if totalResident > cache.clock.capacity {
		t.Errorf("Total resident pages %d exceeds capacity %d",
			totalResident, cache.clock.capacity)
	}
}

func TestHotColdTransitions(t *testing.T) {
	cache := New[interface{}, interface{}](4)

	cache.Put("cold1", "value1")
	cache.Put("cold2", "value2")

	for i := 0; i < 3; i++ {
		cache.Get("cold1")
		cache.Get("cold2")
	}

	totalPages := len(cache.clock.pageMap)
	listSizes := cache.clock.hot.size + cache.clock.cold.size + cache.clock.meta.size

	if totalPages != listSizes {
		t.Errorf("Page map size %d doesn't match list sizes %d", totalPages, listSizes)
	}

	if cache.clock.hot.size > cache.clock.hotCap {
		t.Errorf("Hot list size %d exceeds hot capacity %d",
			cache.clock.hot.size, cache.clock.hotCap)
	}

	residentPages := cache.clock.hot.size + cache.clock.cold.size
	if residentPages > cache.clock.capacity {
		t.Errorf("Resident pages %d exceed total capacity %d",
			residentPages, cache.clock.capacity)
	}
}

func TestSetSize(t *testing.T) {
	cache := New[interface{}, interface{}](5)

	for i := 0; i < 5; i++ {
		cache.Put(i, i*10)
	}

	originalTotal := len(cache.clock.pageMap)

	cache.SetSize(10)
	if cache.clock.capacity != 10 {
		t.Errorf("SetSize(10): capacity = %d, want 10", cache.clock.capacity)
	}

	cache.SetSize(2)
	if cache.clock.capacity != 2 {
		t.Errorf("SetSize(2): capacity = %d, want 2", cache.clock.capacity)
	}

	residentPages := cache.clock.hot.size + cache.clock.cold.size
	if residentPages > 2 {
		t.Errorf("After SetSize(2), resident pages = %d, want <= 2", residentPages)
	}

	cache.Put("new", "value")
	if _, ok := cache.Get("new"); !ok {
		t.Error("Should be able to add items after SetSize")
	}

	_ = originalTotal
}

func TestAdaptiveCapacity(t *testing.T) {
	cache := New[interface{}, interface{}](10)

	initialHotCap := cache.clock.hotCap

	for i := 0; i < 20; i++ {
		cache.Put(i, i)
		if i < 10 {
			for j := 0; j < 3; j++ {
				cache.Get(i)
			}
		}
	}

	if cache.clock.hotCap < 1 {
		t.Error("Hot capacity should be at least 1")
	}

	if cache.clock.hotCap >= cache.clock.capacity {
		t.Errorf("Hot capacity %d should be less than total capacity %d",
			cache.clock.hotCap, cache.clock.capacity)
	}

	if cache.clock.hotCap+cache.clock.coldCap != cache.clock.capacity {
		t.Errorf("Hot capacity %d + cold capacity %d should equal total capacity %d",
			cache.clock.hotCap, cache.clock.coldCap, cache.clock.capacity)
	}

	_ = initialHotCap
}

func TestConcurrency(t *testing.T) {
	cache := New[interface{}, interface{}](100)

	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			cache.Put(i, i*2)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			cache.Get(i)
		}
		done <- true
	}()

	<-done
	<-done

	totalResident := cache.clock.hot.size + cache.clock.cold.size
	if totalResident > cache.clock.capacity {
		t.Errorf("After concurrent access, resident pages %d exceed capacity %d",
			totalResident, cache.clock.capacity)
	}
}

func TestStateTransitions(t *testing.T) {
	cache := New[interface{}, interface{}](4)

	cache.Put("test", "value")

	page := cache.clock.pageMap["test"]
	if page == nil {
		t.Fatal("Page should exist after Put")
	}

	if page.state != stateColdResident {
		t.Errorf("New page state = %v, want %v", page.state, stateColdResident)
	}

	if !page.test {
		t.Error("New page should have test bit set")
	}

	cache.Get("test")

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

		cache.Put("key2", "value2")
		if val, ok := cache.Get("key2"); !ok || val != "value2" {
			t.Errorf("After eviction: got (%v, %v), want (value2, true)", val, ok)
		}

		total := cache.clock.hot.size + cache.clock.cold.size
		if total > 1 {
			t.Errorf("Capacity 1 cache has %d resident pages", total)
		}
	})

	t.Run("nil keys and values", func(t *testing.T) {
		cache := New[interface{}, interface{}](5)

		cache.Put(nil, "value")
		if val, ok := cache.Get(nil); !ok || val != "value" {
			t.Errorf("nil key failed: got (%v, %v)", val, ok)
		}

		cache.Put("key", nil)
		if val, ok := cache.Get("key"); !ok || val != nil {
			t.Errorf("nil value failed: got (%v, %v)", val, ok)
		}
	})
}

func BenchmarkGet(b *testing.B) {
	cache := New[int, int](1000)
	
	for i := 0; i < 1000; i++ {
		cache.Put(i, i*2)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := 0
		for pb.Next() {
			cache.Get(key % 1000)
			key++
		}
	})
}

func BenchmarkPut(b *testing.B) {
	cache := New[int, int](1000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := 0
		for pb.Next() {
			cache.Put(key, key*2)
			key++
		}
	})
}

func BenchmarkMixed(b *testing.B) {
	cache := New[int, int](1000)
	
	for i := 0; i < 500; i++ {
		cache.Put(i, i*2)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		key := 0
		for pb.Next() {
			if key%10 < 7 {
				cache.Get(key % 1000)
			} else {
				cache.Put(key, key*2)
			}
			key++
		}
	})
}

func BenchmarkGetSequential(b *testing.B) {
	cache := New[int, int](1000)
	
	for i := 0; i < 1000; i++ {
		cache.Put(i, i*2)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(i % 1000)
	}
}

func BenchmarkPutSequential(b *testing.B) {
	cache := New[int, int](1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(i, i*2)
	}
}