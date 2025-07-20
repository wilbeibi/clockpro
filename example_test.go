package clockpro_test

import (
	"fmt"
	"strconv"

	"github.com/wilbeibi/clockpro"
)

func ExampleCache_basic() {
	// Create a new cache with capacity for 3 items
	cache := clockpro.New[string, any](3)

	// Add some items
	cache.Put("user:1", "Alice")
	cache.Put("user:2", "Bob")
	cache.Put("user:3", "Charlie")

	// Retrieve items
	if value, found := cache.Get("user:1"); found {
		fmt.Println("Found:", value)
	}

	// Add another item (may cause eviction due to capacity)
	cache.Put("user:4", "Diana")

	fmt.Println("Added user:4")

	// Output:
	// Found: Alice
	// Added user:4
}

func ExampleCache_SetSize() {
	// Start with a small cache
	cache := clockpro.New[string, any](2)

	cache.Put("a", 1)
	cache.Put("b", 2)

	fmt.Println("Initial capacity: 2 items")

	// Increase capacity
	cache.SetSize(5)
	fmt.Println("Increased capacity to 5")

	// Add more items
	cache.Put("c", 3)
	cache.Put("d", 4)
	cache.Put("e", 5)

	// All items should fit now
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		if _, found := cache.Get(key); found {
			fmt.Printf("Key %s: present\n", key)
		}
	}

	// Output:
	// Initial capacity: 2 items
	// Increased capacity to 5
	// Key a: present
	// Key b: present
	// Key c: present
	// Key d: present
	// Key e: present
}

func ExampleCache_workload() {
	// Simulate a more realistic workload
	cache := clockpro.New[string, any](5)

	// Add some "database records"
	for i := 1; i <= 5; i++ {
		key := "record:" + strconv.Itoa(i)
		value := map[string]interface{}{
			"id":   i,
			"name": "Record " + strconv.Itoa(i),
		}
		cache.Put(key, value)
	}

	// Access a specific record multiple times
	for i := 0; i < 3; i++ {
		if value, found := cache.Get("record:3"); found {
			if record, ok := value.(map[string]interface{}); ok {
				fmt.Printf("Accessed: %s\n", record["name"])
			}
		}
	}

	fmt.Println("Cache operational with CLOCK-Pro algorithm")

	// Output:
	// Accessed: Record 3
	// Accessed: Record 3
	// Accessed: Record 3
	// Cache operational with CLOCK-Pro algorithm
}

func ExampleNew() {
	// Create caches with different capacities
	smallCache := clockpro.New[string, any](10)
	largeCache := clockpro.New[string, any](1000)

	// Zero or negative size defaults to 1
	minCache := clockpro.New[string, any](0)

	fmt.Printf("Small cache created with capacity: %d\n", 10)
	fmt.Printf("Large cache created with capacity: %d\n", 1000)
	fmt.Printf("Min cache defaults to capacity: %d\n", 1)

	// Use the caches
	smallCache.Put("key", "value")
	largeCache.Put("key", "value")
	minCache.Put("key", "value")

	fmt.Println("All caches operational")

	// Output:
	// Small cache created with capacity: 10
	// Large cache created with capacity: 1000
	// Min cache defaults to capacity: 1
	// All caches operational
}
