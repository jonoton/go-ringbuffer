package ringbuffer

// Cleanable is an interface for types that require explicit cleanup
// when they are dropped from the RingBuffer (either by being overwritten
// or when Stop() is called).
type Cleanable interface {
	// Cleanup performs any necessary resource release.
	Cleanup()
}

// RingBuffer is a generic, thread-safe ring buffer.
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
	addChan chan T
	getChan chan T
	done    chan struct{}
}

// New creates a new RingBuffer with the given size. If the provided size
// is less than 1, it defaults to a size of 1.
func New[T any](size int) *RingBuffer[T] {
	if size <= 0 {
		size = 1
	}
	rb := &RingBuffer[T]{
		data:    make([]T, size),
		addChan: make(chan T),
		getChan: make(chan T),
		done:    make(chan struct{}),
	}

	go rb.run()

	return rb
}

// Add adds an item to the ring buffer. This operation is thread-safe.
func (rb *RingBuffer[T]) Add(item T) {
	rb.addChan <- item
}

// Get retrieves an item from the ring buffer. This operation is thread-safe.
// It will block until an item is available.
func (rb *RingBuffer[T]) Get() T {
	return <-rb.getChan
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
	// outputChan is used to send items to consumers. It's nil when the buffer
	// is empty to disable the corresponding select case.
	var outputChan chan T
	// currentItem holds the next item to be sent from the tail of the buffer.
	var currentItem T

	for {
		// Before selecting an operation, determine the buffer's state.
		// If the buffer is empty, disable the output channel.
		if !rb.isFull && rb.head == rb.tail {
			outputChan = nil
		} else {
			// Otherwise, prepare the next item for retrieval and enable the output channel.
			currentItem = rb.data[rb.tail]
			outputChan = rb.getChan
		}

		select {
		case item := <-rb.addChan:
			// If the buffer is full, the item at the current head position
			// is about to be overwritten. Clean it up if it's Cleanable.
			if rb.isFull {
				// We use 'any' to convert the generic T to an interface
				// type so we can perform the type assertion.
				if cleanable, ok := any(rb.data[rb.head]).(Cleanable); ok {
					cleanable.Cleanup()
				}
			}

			// A producer sent a new item. Store it at the head.
			rb.data[rb.head] = item
			rb.head = (rb.head + 1) % len(rb.data)

			// If the buffer was already full, the oldest item was just
			// overwritten, so we also need to advance the tail to follow the head.
			if rb.isFull {
				rb.tail = rb.head
			} else if rb.head == rb.tail {
				// The buffer has just become full.
				rb.isFull = true
			}

		case outputChan <- currentItem:
			// An item was successfully sent to a consumer. Advance the tail.
			rb.tail = (rb.tail + 1) % len(rb.data)
			// The buffer cannot be full after a successful get.
			rb.isFull = false

		case <-rb.done:
			// The stop signal was received. Clean up any remaining items
			// in the buffer before exiting the goroutine.
			if rb.isFull {
				// If the buffer is full, every slot has an item to check.
				for i := 0; i < len(rb.data); i++ {
					if cleanable, ok := any(rb.data[i]).(Cleanable); ok {
						cleanable.Cleanup()
					}
				}
			} else {
				// If not full, iterate from the tail to the head.
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
