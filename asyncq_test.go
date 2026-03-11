package asyncq

import (
	"log"
	"os"
	"testing"
	"time"
)

var (
	logger    = log.New(os.Stdout, "", 0)
	fakeMutex = &FakeMutex{true}
)

type FakeMutex struct {
	tryLockResult bool
}

func (m *FakeMutex) Lock() {}

func (m *FakeMutex) TryLock() bool {
	return m.tryLockResult
}

func (m *FakeMutex) Unlock() {}

func TestAsyncDoubleQueue_exchangeQueuesIsOk_WhenOutputQIsEmpty(t *testing.T) {

	var initCap = 10
	var inputQueue = make([]func(), 0, initCap)
	for i := 0; i < initCap; i++ {
		inputQueue = append(inputQueue, func() {})
	}

	var queue = &AsyncDoubleQueue{
		inputQueue:  inputQueue,
		outputQueue: make([]func(), 0, initCap),
		mutex:       fakeMutex,
	}

	var result = queue.exchangeQueues()

	if !result {
		t.Fatalf("input queue is empty")
	}
	if len(queue.outputQueue) != initCap {
		t.Fatalf("output queue length is not equal to initCap")
	}
	if len(queue.inputQueue) != 0 {
		t.Fatalf("input queue length is not equal to 0")
	}
}

func TestAsyncDoubleQueue_exchangeQueuesIsOk_When2Executions(t *testing.T) {

	var initCap = 10
	var inputQueue = make([]func(), 0, initCap)
	for i := 0; i < initCap; i++ {
		inputQueue = append(inputQueue, func() {})
	}

	var queue = &AsyncDoubleQueue{
		inputQueue:  inputQueue,
		outputQueue: make([]func(), 0, initCap),
		mutex:       fakeMutex,
	}

	queue.exchangeQueues()

	for i := 0; i < 5; i++ {
		queue.inputQueue = append(queue.inputQueue, func() {})
	}

	var result = queue.exchangeQueues()

	if !result {
		t.Fatalf("input queue is empty")
	}
	if len(queue.outputQueue) != 5 {
		t.Fatalf("output queue length is not equal to 5")
	}
	if len(queue.inputQueue) != 0 {
		t.Fatalf("input queue length is not equal to 0")
	}
}

func TestAsyncDoubleQueue_EnqueueIsOk_WhenNonNilInput(t *testing.T) {

	var initCap = 10
	var syn = make(chan bool)

	var queue = &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		syn:         syn,
		mutex:       fakeMutex,
	}

	go func() {
		<-syn
	}()
	queue.Enqueue(func() {})

	if len(queue.inputQueue) != 1 {
		t.Fatalf("input queue length is not equal to 1")
	}
}

func TestAsyncDoubleQueue_EnqueueIsOk_WhenNilInput(t *testing.T) {

	var initCap = 10

	var queue = &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		mutex:       fakeMutex,
	}

	queue.Enqueue(nil)

	if len(queue.inputQueue) != 0 {
		t.Fatalf("input queue length is not equal to 0")
	}
}

func TestAsyncDoubleQueue_CloseIsOk(t *testing.T) {

	var initCap = 10
	var syn = make(chan bool)

	var queue = &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		syn:         syn,
		mutex:       fakeMutex,
	}

	go func() {
		<-syn
	}()
	queue.Close()

	if len(queue.inputQueue) != 1 {
		t.Fatalf("input queue length is not equal to 1")
	}
	if queue.inputQueue[0] != nil {
		t.Fatalf("input queue is not closed")
	}
}

func TestAsyncDoubleQueue_dequeueIsOk_WhenSingleTask(t *testing.T) {

	var initCap = 10
	var queue = &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		mutex:       fakeMutex,
	}

	var value = new(int)
	*value = 0
	queue.inputQueue = append(queue.inputQueue, func() { *value++ })

	queue.exchangeQueues()

	task, isEmpty := queue.dequeue()

	if isEmpty {
		t.Fatalf("output queue is empty")
	}
	task()

	if *value != 1 {
		t.Fatalf("value is not equal to 1")
	}

	task, isEmpty = queue.dequeue()

	if !isEmpty {
		t.Fatalf("output queue is not empty")
	}
	if task != nil {
		t.Fatalf("task is not nil")
	}
}

func TestAsyncDoubleQueue_dequeueIsOk_WhenNoTasks(t *testing.T) {

	var initCap = 10
	var queue = &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		mutex:       fakeMutex,
	}

	task, isEmpty := queue.dequeue()

	if !isEmpty {
		t.Fatalf("output queue is not empty")
	}
	if task != nil {
		t.Fatalf("task is not nil")
	}
}

func TestAsyncDoubleQueue_RunEventLoopIsOk(t *testing.T) {

	var initCap = 10
	var queue = NewAsyncDoubleQueue(initCap, logger)
	var value = new(int)
	*value = 0

	go queue.RunEventLoop()
	defer queue.Close()

	queue.Enqueue(func() { *value++ })
	queue.Enqueue(func() { *value++ })
	queue.Enqueue(func() { *value++ })
	queue.Enqueue(func() { *value++ })
	queue.Enqueue(func() { *value++ })

	time.Sleep(1 * time.Second)
	if *value != 5 {
		t.Fatalf("value = %d, expected 5", *value)
	}
}
