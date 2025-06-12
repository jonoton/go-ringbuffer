/*
Package ringbuffer provides a generic, thread-safe, fixed-size circular buffer.

The RingBuffer is designed for concurrent producer-consumer scenarios. It allows
multiple goroutines to safely add and retrieve items. When the buffer reaches its
capacity, new items will overwrite the oldest ones.

It uses Go's generics, allowing it to store elements of any type. Thread safety is
achieved internally by using channels to serialize access.

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

Non-Blocking and Batch Operations:

The buffer also provides non-blocking and batch retrieval methods.

TryGet attempts to retrieve an item without blocking. If the buffer is empty,
it returns the zero value for the type and false.

	if item, ok := rb.TryGet(); ok {
		// An item was successfully retrieved and can be processed.
		fmt.Printf("Got item: %v\n", item)
	}

GetAll atomically retrieves and removes all items from the buffer, returning
them as a slice. This operation does not block.

	allItems := rb.GetAll()
	fmt.Printf("Retrieved %d items at once.\n", len(allItems))

Automatic Cleanup:

Types that require cleanup (e.g., to release file handles or network connections)
can implement the `Cleanable` interface. The `Cleanup()` method will be called
automatically when an item is overwritten or when `Stop()` is called on the buffer.

	type MyResource struct {
		// ... fields
	}

	func (r *MyResource) Cleanup() {
		fmt.Println("Cleaning up MyResource!")
		// ... release resources here
	}

	rb := ringbuffer.New[*MyResource](5)
	rb.Add(&MyResource{}) // This item's Cleanup() will be called later.
*/
package ringbuffer
