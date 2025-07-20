# CLOCK-Pro Cache

A high-performance cache replacement algorithm implementation in Go.

CLOCK-Pro is an advanced replacement policy that adapts to both recency and frequency patterns, making it effective for workloads with mixed access characteristics.

## Installation

```bash
go get github.com/wilbeibi/clockpro
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/wilbeibi/clockpro"
)

func main() {
    cache := clockpro.New(1000)
    
    cache.Put("user:123", map[string]string{
        "name": "John Doe",
        "role": "admin",
    })
    
    if value, found := cache.Get("user:123"); found {
        fmt.Printf("Found user: %v\n", value)
    }
}
```

## API

### Creating a Cache

```go
cache := clockpro.New(capacity)  // capacity must be > 0
```

### Basic Operations

```go
// Store a key-value pair
cache.Put(key, value)

// Retrieve a value
value, found := cache.Get(key)

// Adjust cache size
cache.SetSize(newCapacity)
```

## Features

- **Adaptive**: Automatically balances between recency and frequency
- **Scan-resistant**: Handles sequential access patterns well
- **Thread-safe**: Concurrent access supported
- **Memory-efficient**: Tracks non-resident metadata for better decisions
- **Dynamic sizing**: Capacity can be adjusted at runtime

## Performance

Benchmarks show competitive performance with traditional LRU:

| Operation | Throughput | Latency |
|-----------|------------|---------|
| Get | ~40M ops/sec | ~30 ns |
| Put | ~15M ops/sec | ~75 ns |
| Mixed (70/30) | ~18M ops/sec | ~77 ns |

Run benchmarks:

```bash
go test -bench=.
```

## Algorithm

CLOCK-Pro maintains three circular lists:

- **Hot list**: Frequently accessed pages
- **Cold list**: Recently accessed pages  
- **Meta list**: Non-resident page metadata

The algorithm adapts the hot/cold boundary based on access patterns, providing better hit rates than fixed-ratio policies.

## Use Cases

- Database buffer pools
- Web application caches
- File system page caches
- Any workload mixing hot data with sequential scans

## Testing

```bash
go test -v        # Run tests
go test -race     # Test with race detector
go test -cover    # Generate coverage report
```

## License

MIT