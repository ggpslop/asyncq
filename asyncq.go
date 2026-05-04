package asyncq

import (
	"log"
	"os"
	"sync"
)

// Constants and global variables

const defaultCap = 10

// Types, structs and interfaces

type WaitFunc func()

type Mutex interface {
	Lock()
	TryLock() bool
	Unlock()
}

// AsyncQ is an interface for any AsyncQ implementation. Allows
// queuing, closing the queue, and starting the Event Loop.
// Tasks are simply parameterless procedures, because you can
// use closures that capture external context variables.
type AsyncQ interface {

	// Enqueue a new task in queue.
	Enqueue(task func())

	// TryEnqueue enqueue a new task in queue if it's possible. The 'possibility'
	// must be decided by the specific implementation. For example, can be
	// useful when a lock mechanism is used, like a mutex.
	TryEnqueue(task func()) bool

	// Close the queue and no more tasks can be enqueued. It returns a wait
	// function to be used to graceful shutting down the queue.
	Close() WaitFunc

	// RunEventLoop start the event loop. It must be started in a new goroutine.
	RunEventLoop()
}

// AsyncDoubleQueue uses two queues, one in for enqueue operations (inputQueue)
// and another one for dequeues operations (outputQueue).
//
//   - A single mutex is used to lock only the inputQueue. The event loop acquire the
//     lock only in one occasion: when outputQueue is empty and exchanges the two queues.
//   - Use a channel to put the event loop to sleep or to wake-up it.
//   - The event loop is 'panic' safe.
//   - A nil task closes the event loop (but not the channel, so it's safe to only use Close).
type AsyncDoubleQueue struct {
	inputQueue  []func()
	outputQueue []func()
	outputIdx   int
	syn         chan bool
	mutex       Mutex
	logger      *log.Logger
}

// NewAsyncDoubleQueue returns a new heap allocated AsyncDoubleQueue.
//
//   - If 'initCap' is less the 'defaultCap', 'defaultCap' is used as initial capacity.
//   - If 'logger' is nil, plain STDOUT is used.
func NewAsyncDoubleQueue(initCap int, logger *log.Logger) *AsyncDoubleQueue {

	if logger == nil {
		logger = log.New(os.Stdout, "", 0)
	}
	if initCap < defaultCap {
		initCap = defaultCap
	}

	return &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		outputIdx:   0,
		syn:         make(chan bool),
		mutex:       &sync.Mutex{},
		logger:      logger,
	}
}

func (aq *AsyncDoubleQueue) Enqueue(task func()) {

	if task == nil {
		return
	}

	aq.mutex.Lock()
	defer aq.mutex.Unlock()

	aq.inputQueue = append(aq.inputQueue, task)

	select {
	case aq.syn <- true:
		// Wake up the event loop.
	default:
		// Event loop is already awake.
	}
}

// TryEnqueue returns 'true' when the enqueue is done, 'false' otherwise.
func (aq *AsyncDoubleQueue) TryEnqueue(task func()) bool {

	if task == nil {
		return true
	}

	if !aq.mutex.TryLock() {
		return false
	}
	defer aq.mutex.Unlock()

	aq.inputQueue = append(aq.inputQueue, task)

	select {
	case aq.syn <- true:
		// Wake up the event loop.
	default:
		// Event loop is already awake.
	}

	return true
}

// Close the event loop and the channel.
// It uses sync.WaitGroup with counter = 1 to create a 'wait' function,
// that it then returns. To close the event loop, it adds 2 task to the
// inputQueue, a task with sync.WaitGroup.Done and a nil task that is
// recognized by the event loop logic.
func (aq *AsyncDoubleQueue) Close() WaitFunc {

	aq.mutex.Lock()
	defer aq.mutex.Unlock()

	var wg sync.WaitGroup // escaped to heap.
	wg.Add(1)

	aq.inputQueue = append(aq.inputQueue, func() { wg.Done() }, nil)

	select {
	case aq.syn <- true:
		// Wake up the event loop.
	default:
		// Event loop is already awake.
	}

	close(aq.syn)

	return wg.Wait
}

// exchangeQueues exchanges the input and output queues. This method is
// the only one the acquires the lock for the inputQueue in the event loop.
// Returns 'false' when the input queue is empty, 'true' otherwise.
func (aq *AsyncDoubleQueue) exchangeQueues() bool {

	aq.mutex.Lock()
	defer aq.mutex.Unlock()

	if len(aq.inputQueue) == 0 {
		return false
	}

	aq.inputQueue, aq.outputQueue = aq.outputQueue[:0], aq.inputQueue

	return true
}

// dequeue does not acquire the lock unless the outputQueue is empty,
// but acquires the lock only for the time needed to exchange the queues.
// Returns the task dequeued plus 'false', or a nil task plus 'true'
// when the queue is empty.
func (aq *AsyncDoubleQueue) dequeue() (func(), bool) {

	if aq.outputIdx >= len(aq.outputQueue) {
		if !aq.exchangeQueues() {
			return nil, true
		}
		aq.outputIdx = 0
	}

	task := aq.outputQueue[aq.outputIdx]
	aq.outputIdx++

	return task, false
}

func (aq *AsyncDoubleQueue) RunEventLoop() {

	aq.logger.Println("AsyncQ - starting the Async Event Loop")

	for aq.safeRunEventLoop() {
		// Empty body. Panic can't stop the event loop.
	}
}

func (aq *AsyncDoubleQueue) safeRunEventLoop() (result bool) {

	// Set to 'true' immediately to prevent the loop from
	// ending prematurely in case of panic.
	result = true

	defer func() {
		if r := recover(); r != nil {
			aq.logger.Println("AsyncQ - recover from panic.")
		}
	}()

	task, isEmpty := aq.dequeue()
	if isEmpty {
		// Wait and sleep.
		<-aq.syn
		return
	}

	if task == nil {
		// Set 'false' before calling the logger, so the closing of
		// the queue is not bypassed in case of panic.
		result = false
		aq.logger.Println("AsyncQ - closing the Async Event Loop")
		return
	}

	task()
	return
}
