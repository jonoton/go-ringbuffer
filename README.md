# go-ringbuffer
Package `ringbuffer` provides a generic, thread-safe, fixed-size circular buffer for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/jonoton/go-ringbuffer.svg)](https://pkg.go.dev/github.com/jonoton/go-ringbuffer)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonoton/go-ringbuffer?)](https://goreportcard.com/report/github.com/jonoton/go-ringbuffer)

The `RingBuffer` is designed for concurrent producer-consumer scenarios. It allows multiple goroutines to safely add and retrieve items without requiring manual locking. When the buffer reaches its capacity, new items will overwrite the oldest ones.

It uses Go's generics, allowing it to store elements of any type. Thread safety is achieved internally by using channels to serialize access, which also allows the `Get()` operation to block until an item is available.

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
	item2 := rb.g.Get() // "world"
	fmt.Println(item1, item2)
}()
```