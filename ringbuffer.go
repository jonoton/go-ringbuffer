package ringbuffer

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

// New creates a new RingBuffer with the given size. The size must be positive.
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
			// The stop signal was received. Exit the goroutine.
			return
		}
	}
}
