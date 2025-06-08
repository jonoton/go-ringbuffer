/*
Package ringbuffer provides a generic, thread-safe, fixed-size circular buffer.

The RingBuffer is designed for concurrent producer-consumer scenarios. It allows
multiple goroutines to safely add and retrieve items. When the buffer reaches its
capacity, new items will overwrite the oldest ones.

It uses Go's generics, allowing it to store elements of any type. Thread safety is
achieved internally by using channels to serialize access, eliminating the need for
explicit locking by the user.

Usage:

Create a new ring buffer of a specific type and size:

	rb := ringbuffer.New[string](10)
	defer rb.Stop() // Clean up the background goroutine when done.

Add items from one or more goroutines:

	go func() {
		rb.Add("hello")
		rb.Add("world")
	}()

Retrieve items from one or more goroutines. The Get() call will block
until an item is available:

	go func() {
		item1 := rb.Get() // "hello"
		item2 := rb.Get() // "world"
		fmt.Println(item1, item2)
	}()
*/
package ringbuffer
