package ringbuffer

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestNew verifies the initial state of a new RingBuffer.
func TestNew(t *testing.T) {
	size := 10
	rb := New[int](size)
	defer rb.Stop()

	if rb == nil {
		t.Fatal("New() returned nil")
	}
	if len(rb.data) != size {
		t.Errorf("expected buffer size %d, got %d", size, len(rb.data))
	}
	if rb.head != 0 {
		t.Errorf("expected initial head to be 0, got %d", rb.head)
	}
	if rb.tail != 0 {
		t.Errorf("expected initial tail to be 0, got %d", rb.tail)
	}
	if rb.isFull {
		t.Error("expected initial isFull to be false")
	}
}

// TestAddAndGet tests basic addition and retrieval of items.
func TestAddAndGet(t *testing.T) {
	rb := New[string](3)
	defer rb.Stop()

	rb.Add("hello")
	item := rb.Get()
	if item != "hello" {
		t.Errorf("expected to get 'hello', got '%s'", item)
	}

	rb.Add("world")
	rb.Add("foo")
	rb.Add("bar") // "world" should now be at the tail

	if got := rb.Get(); got != "world" {
		t.Errorf("expected to get 'world', got '%s'", got)
	}
	if got := rb.Get(); got != "foo" {
		t.Errorf("expected to get 'foo', got '%s'", got)
	}
	if got := rb.Get(); got != "bar" {
		t.Errorf("expected to get 'bar', got '%s'", got)
	}
}

// TestOverwrite verifies that the buffer correctly overwrites old items when full.
func TestOverwrite(t *testing.T) {
	size := 3
	rb := New[int](size)
	defer rb.Stop()

	// Fill the buffer
	for i := 0; i < size; i++ {
		rb.Add(i)
	}

	// Overwrite the first element (0) with a new one (3)
	rb.Add(3)

	// Overwrite the second element (1) with (4)
	rb.Add(4)

	// The buffer should now contain [3, 4, 2] with the tail pointing at 2.
	// Let's verify the order of retrieval.
	expected := []int{2, 3, 4}
	for _, want := range expected {
		if got := rb.Get(); got != want {
			t.Errorf("expected to get %d, got %d", want, got)
		}
	}
}

// TestGetBlocksUntilAdd confirms that Get() blocks until an item is available.
func TestGetBlocksUntilAdd(t *testing.T) {
	rb := New[int](2)
	defer rb.Stop()

	c := make(chan int)
	go func() {
		// This will block until an item is added.
		item := rb.Get()
		c <- item
	}()

	// Give the goroutine a moment to start and block on Get().
	time.Sleep(20 * time.Millisecond)

	// Now, add an item to unblock the goroutine.
	rb.Add(123)

	select {
	case item := <-c:
		if item != 123 {
			t.Errorf("expected to get 123, got %d", item)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Get() did not unblock in time")
	}
}

// TestStop ensures the background goroutine terminates gracefully.
func TestStop(t *testing.T) {
	initialGoRoutines := runtime.NumGoroutine()

	rb := New[int](5)
	rb.Add(1)
	rb.Stop()

	// Give a moment for the goroutine to terminate.
	time.Sleep(50 * time.Millisecond)

	finalGoRoutines := runtime.NumGoroutine()

	// If Stop() worked, the number of goroutines should be back to the initial count.
	// The original check 'finalGoRoutines >= initialGoRoutines' was incorrect, as it
	// would fail when final == initial (the success case).
	// A small grace is given for other potential runtime goroutines.
	if finalGoRoutines > initialGoRoutines {
		t.Errorf("Stop() did not terminate the background goroutine; initial: %d, final: %d", initialGoRoutines, finalGoRoutines)
	}
}

// TestConcurrentAddAndGet tests the thread-safety of Add and Get operations.
func TestConcurrentAddAndGet(t *testing.T) {
	rb := New[int](100) // A larger buffer to handle concurrency
	defer rb.Stop()

	numProducers := 5
	numConsumers := 5
	itemsPerProducer := 20
	totalItems := numProducers * itemsPerProducer

	var wg sync.WaitGroup

	// Producers
	wg.Add(numProducers)
	for i := 0; i < numProducers; i++ {
		go func(producerID int) {
			defer wg.Done()
			for j := 0; j < itemsPerProducer; j++ {
				// Each item is unique to avoid accidental duplicates.
				item := producerID*itemsPerProducer + j
				rb.Add(item)
			}
		}(i)
	}

	// Consumers
	results := make(chan int, totalItems)
	wg.Add(numConsumers)
	for i := 0; i < numConsumers; i++ {
		go func() {
			defer wg.Done()
			// Each consumer will pull a portion of the total items.
			for j := 0; j < totalItems/numConsumers; j++ {
				results <- rb.Get()
			}
		}()
	}

	wg.Wait()
	close(results)

	// Verification
	receivedMap := make(map[int]bool)
	for item := range results {
		if receivedMap[item] {
			t.Errorf("duplicate item received: %d", item)
		}
		receivedMap[item] = true
	}

	if len(receivedMap) != totalItems {
		t.Errorf("expected to receive %d unique items, but got %d", totalItems, len(receivedMap))
	}
}

// Test a struct type to ensure generics are working correctly.
type testStruct struct {
	ID   int
	Name string
}

func TestStructType(t *testing.T) {
	rb := New[testStruct](2)
	defer rb.Stop()

	s1 := testStruct{ID: 1, Name: "one"}
	s2 := testStruct{ID: 2, Name: "two"}

	rb.Add(s1)
	rb.Add(s2)

	item1 := rb.Get()
	if !reflect.DeepEqual(item1, s1) {
		t.Errorf("expected %+v, got %+v", s1, item1)
	}

	item2 := rb.Get()
	if !reflect.DeepEqual(item2, s2) {
		t.Errorf("expected %+v, got %+v", s2, item2)
	}
}

// Test an empty buffer scenario
func TestEmptyBufferGet(t *testing.T) {
	rb := New[int](1)
	defer rb.Stop()

	done := make(chan bool)

	go func() {
		// This Get() will block as nothing is added
		rb.Get()
		// If Get ever returns (which it shouldn't in this test's timeframe),
		// it would send to this channel, causing the test to fail.
		done <- true
	}()

	select {
	case <-done:
		t.Fatal("Get() returned on an empty buffer without an Add()")
	case <-time.After(50 * time.Millisecond):
		// This is the expected outcome: Get() is still blocking.
		fmt.Println("TestEmptyBufferGet: Get() correctly blocked.")
	}
}
