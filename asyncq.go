package asyncq

import (
	"log"
	"os"
	"sync"
)

// Constants and global variables

const defaultCap = 10

// Structs and interfaces

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

	// Close the queue. No more tasks can be enqueued.
	Close()

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
		// wake up the event loop
	default:
		// event loop is already awake
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
		// wake up the event loop
	default:
		// event loop is already awake
	}

	return true
}

// Close the event loop (add a nil task to the inputQueue) and the channel.
func (aq *AsyncDoubleQueue) Close() {

	aq.mutex.Lock()
	defer aq.mutex.Unlock()

	aq.inputQueue = append(aq.inputQueue, nil)

	select {
	case aq.syn <- true:
		// wake up the event loop
	default:
		// event loop is already awake
	}

	close(aq.syn)
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

	tmp := aq.inputQueue
	aq.inputQueue = aq.outputQueue[:0]
	aq.outputQueue = tmp

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

	aq.logger.Println("Starting the Async Event Loop")

	for aq.safeRunEventLoop() {
		// empty body
		// panic can't stop the event loop
	}
}

func (aq *AsyncDoubleQueue) safeRunEventLoop() bool {

	defer func() {
		if r := recover(); r != nil {
			aq.logger.Println("Recover from panic.")
		}
	}()

	task, isEmpty := aq.dequeue()
	if isEmpty {
		// wait and sleep
		<-aq.syn
		return true
	}

	if task == nil {
		aq.logger.Println("Closing the Async Event Loop")
		return false
	}

	task()
	return true
}
