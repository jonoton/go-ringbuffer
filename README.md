# go-ringbuffer
Package `ringbuffer` provides a generic, thread-safe, fixed-size circular buffer for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/jonoton/go-ringbuffer.svg)](https://pkg.go.dev/github.com/jonoton/go-ringbuffer)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonoton/go-ringbuffer?)](https://goreportcard.com/report/github.com/jonoton/go-ringbuffer)

The `RingBuffer` is designed for concurrent producer-consumer scenarios. It allows multiple goroutines to safely add and retrieve items without requiring manual locking. When the buffer reaches its capacity, new items will overwrite the oldest ones.

It uses Go's generics, allowing it to store elements of any type. Thread safety is
achieved internally by using channels to serialize access.

## Usage

Create a new ring buffer of a specific type and size:

```go
rb := ringbuffer.New[string](10)
defer rb.Stop() // Clean up the background goroutine when done.
```

Add items from one or more goroutines:

```go
go func() {
	rb.Add("hello")
	rb.Add("world")
}()
```

Retrieve items from one or more goroutines. The Get() call will block until an item is available:

```go
go func() {
	item1 := rb.Get() // "hello"
	item2 := rb.Get() // "world"
	fmt.Println(item1, item2)
}()
```

Automatic Cleanup:

Types that require cleanup (e.g., to release file handles or network connections)
can implement the `Cleanable` interface. The `Cleanup()` method will be called
automatically when an item is overwritten or when `Stop()` is called on the buffer.

```go
type MyResource struct {
	// ... fields
}

func (r *MyResource) Cleanup() {
	fmt.Println("Cleaning up MyResource!")
	// ... release resources here
}

rb := ringbuffer.New[*MyResource](5)
rb.Add(&MyResource{}) // This item's Cleanup() will be called later.
```
