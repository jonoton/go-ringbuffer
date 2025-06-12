package ringbuffer

// Cleanable is an interface for types that require explicit cleanup
// when they are dropped from the RingBuffer (either by being overwritten
// or when Stop() is called).
type Cleanable interface {
	// Cleanup performs any necessary resource release.
	Cleanup()
}

// tryGetResponse is a private struct used to send the result
// of a TryGet operation back to the caller.
type tryGetResponse[T any] struct {
	item T
	ok   bool
}

// RingBuffer is a generic, thread-safe ring buffer. It is designed for
// concurrent producer-consumer scenarios and uses channels to serialize access.
type RingBuffer[T any] struct {
	data []T
	// head is the index where the next item will be added.
	head int
	// tail is the index of the next item to be retrieved.
	tail int
	// isFull is true when the buffer is at capacity. This helps distinguish
	// between a full and an empty buffer when head == tail.
	isFull bool

	// Channels for thread-safe operations
	addChan    chan T
	getChan    chan T
	tryGetChan chan chan tryGetResponse[T] // Channel for non-blocking get requests
	getAllChan chan chan []T               // Channel for getting all items
	done       chan struct{}
}

// New creates a new RingBuffer with the given size. If the provided size
// is less than 1, it defaults to a size of 1.
func New[T any](size int) *RingBuffer[T] {
	if size <= 0 {
		size = 1
	}
	rb := &RingBuffer[T]{
		data:       make([]T, size),
		addChan:    make(chan T),
		getChan:    make(chan T),
		tryGetChan: make(chan chan tryGetResponse[T]),
		getAllChan: make(chan chan []T),
		done:       make(chan struct{}),
	}

	go rb.run()

	return rb
}

// Add adds an item to the ring buffer. This operation is thread-safe.
func (rb *RingBuffer[T]) Add(item T) {
	rb.addChan <- item
}

// Get retrieves an item from the ring buffer. This operation is thread-safe and
// will block until an item is available.
func (rb *RingBuffer[T]) Get() T {
	return <-rb.getChan
}

// TryGet attempts to retrieve an item from the ring buffer without blocking.
// If the buffer is not empty, it returns the oldest item and true.
// If the buffer is empty, it returns the zero value for the type and false.
func (rb *RingBuffer[T]) TryGet() (T, bool) {
	respChan := make(chan tryGetResponse[T], 1)
	rb.tryGetChan <- respChan
	resp := <-respChan
	return resp.item, resp.ok
}

// GetAll retrieves and removes all items currently in the buffer, returning them
// as a slice ordered from oldest to newest. It does not block. The buffer will
// be empty after this call. This operation does not trigger the Cleanup method
// on the retrieved items.
func (rb *RingBuffer[T]) GetAll() []T {
	respChan := make(chan []T, 1)
	rb.getAllChan <- respChan
	return <-respChan
}

// Stop gracefully shuts down the ring buffer's background goroutine.
// It will also call Cleanup() on any remaining items that implement the Cleanable interface.
func (rb *RingBuffer[T]) Stop() {
	close(rb.done)
}

// run is the core loop that serializes access to the buffer.
// It uses a nil channel to disable the 'get' case when the buffer is empty,
// preventing deadlocks and ensuring the 'done' signal is always received.
func (rb *RingBuffer[T]) run() {
	var outputChan chan T
	var currentItem T

	for {
		// Before the select, determine if the buffer has items to send.
		if !rb.isFull && rb.head == rb.tail {
			// Buffer is empty, so disable the 'Get' case by setting the channel to nil.
			// A send to a nil channel blocks forever.
			outputChan = nil
		} else {
			// Buffer has items, so prepare the next item and enable the 'Get' case.
			currentItem = rb.data[rb.tail]
			outputChan = rb.getChan
		}

		select {
		case item := <-rb.addChan:
			// An item was added.
			if rb.isFull {
				// If full, the item at the head is about to be overwritten.
				// We must clean it up first if it implements Cleanable.
				if cleanable, ok := any(rb.data[rb.head]).(Cleanable); ok {
					cleanable.Cleanup()
				}
			}
			// Store the new item.
			rb.data[rb.head] = item
			rb.head = (rb.head + 1) % len(rb.data)
			if rb.isFull {
				// If we overwrote an item, the tail must follow the head.
				rb.tail = rb.head
			} else if rb.head == rb.tail {
				// The buffer is now full.
				rb.isFull = true
			}

		case outputChan <- currentItem:
			// An item was successfully sent to a 'Get' consumer.
			rb.tail = (rb.tail + 1) % len(rb.data)
			rb.isFull = false

		case respChan := <-rb.tryGetChan:
			// A non-blocking 'TryGet' request was received.
			if !rb.isFull && rb.head == rb.tail {
				// Buffer is empty, respond immediately.
				respChan <- tryGetResponse[T]{ok: false}
			} else {
				// Buffer has an item, send it and advance the tail.
				itemToReturn := rb.data[rb.tail]
				rb.tail = (rb.tail + 1) % len(rb.data)
				rb.isFull = false
				respChan <- tryGetResponse[T]{item: itemToReturn, ok: true}
			}

		case respChan := <-rb.getAllChan:
			// A 'GetAll' request was received.
			if !rb.isFull && rb.head == rb.tail {
				respChan <- nil
				continue
			}

			// Copy items from tail to head into a new slice.
			var items []T
			if rb.isFull {
				items = make([]T, len(rb.data))
				copied := copy(items, rb.data[rb.tail:])
				copy(items[copied:], rb.data[:rb.tail])
			} else {
				if rb.head > rb.tail {
					items = make([]T, rb.head-rb.tail)
					copy(items, rb.data[rb.tail:rb.head])
				} else { // Wrapped
					items = make([]T, len(rb.data)-rb.tail+rb.head)
					copied := copy(items, rb.data[rb.tail:])
					copy(items[copied:], rb.data[:rb.head])
				}
			}

			// Reset buffer state to empty.
			rb.tail = rb.head
			rb.isFull = false

			respChan <- items

		case <-rb.done:
			// A 'Stop' request was received.
			// Clean up any remaining items in the buffer before exiting.
			if rb.isFull {
				for i := 0; i < len(rb.data); i++ {
					if cleanable, ok := any(rb.data[i]).(Cleanable); ok {
						cleanable.Cleanup()
					}
				}
			} else {
				for i := rb.tail; i != rb.head; i = (i + 1) % len(rb.data) {
					if cleanable, ok := any(rb.data[i]).(Cleanable); ok {
						cleanable.Cleanup()
					}
				}
			}
			return
		}
	}
}
