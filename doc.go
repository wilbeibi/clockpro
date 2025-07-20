// Package clockpro implements the CLOCK-Pro cache replacement algorithm.
//
// CLOCK-Pro is an advanced replacement algorithm that maintains three circular lists:
// hot (H), resident cold (R), and non-resident cold (M). It adapts to both recency
// and frequency of access patterns, making it effective for various workloads.
//
// The algorithm tracks page states (hot, cold, cold-resident) and uses hand movements
// to manage adaptive sizing and eviction decisions. Hot pages are frequently accessed
// and have higher retention priority, while cold pages are managed for recency.
//
// Basic usage:
//
//	cache := clockpro.New(1000) // capacity of 1000 items
//	cache.Put("key1", "value1")
//	value, found := cache.Get("key1")
//	cache.SetSize(2000) // adjust capacity
//
// The implementation follows the CLOCK-Pro design from the 2005 USENIX ATC paper
// by Song Jiang & Xiaodong Zhang.
package clockpro
