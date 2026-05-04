package asyncq

import (
	"log"
	"os"
	"runtime"
	"sync"
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

func TestAsyncDoubleQueue_Close(t *testing.T) {

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

	if len(queue.inputQueue) != 2 {
		t.Fatalf("input queue length is not equal to 2")
	}
	if queue.inputQueue[1] != nil {
		t.Fatalf("input queue is not closed")
	}
}

func TestAsyncDoubleQueue_CloseWait(t *testing.T) {

	var queue = NewAsyncDoubleQueue(10, logger)
	go queue.RunEventLoop()

	const total = 100
	var value = 0
	var task = func() { value++ }

	for i := 0; i < total; i++ {
		queue.Enqueue(task)
	}
	var queueWait = queue.Close()

	queueWait()
	if value != total {
		t.Fatalf("value is not equal to %d", total)
	}
	_, closed := <-queue.syn
	if closed {
		t.Fatalf("queue was not closed")
	}
}

func TestAsyncDoubleQueue_EnqueueAfterClose_ShouldPanic(t *testing.T) {

	var queue = NewAsyncDoubleQueue(10, logger)
	go queue.RunEventLoop()

	const total = 100
	var value = 0
	var task = func() { value++ }

	for i := 0; i < total; i++ {
		queue.Enqueue(task)
	}
	var queueWait = queue.Close()

	defer func() {

		queueWait()
		if value != total {
			t.Fatalf("value is not equal to %d", total)
		}

		var r = recover()
		if r == nil {
			t.Fatal("should have panicked")
		}
	}()

	queue.Enqueue(task) // panic because the queue is closed.
}

func TestAsyncDoubleQueue_dequeueIsOk_WhenSingleTask(t *testing.T) {

	var initCap = 10
	var queue = &AsyncDoubleQueue{
		inputQueue:  make([]func(), 0, initCap),
		outputQueue: make([]func(), 0, initCap),
		mutex:       fakeMutex,
	}

	var value = 0
	queue.inputQueue = append(queue.inputQueue, func() { value++ })

	queue.exchangeQueues()

	task, isEmpty := queue.dequeue()

	if isEmpty {
		t.Fatalf("output queue is empty")
	}
	task()

	if value != 1 {
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

func TestAsyncDoubleQueue_PanicIsRecovered(t *testing.T) {

	var queue = NewAsyncDoubleQueue(10, logger)
	go queue.RunEventLoop()
	defer queue.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	var value = 0

	queue.Enqueue(func() { panic("I'm in panic!") })
	queue.Enqueue(func() {
		value = value + 1
		wg.Done()
	})
	wg.Wait()
	if value != 1 {
		t.Fatalf("value is not equal to 1")
	}
}

func TestAsyncDoubleQueue_IncrementSingleSharedCounter(t *testing.T) {

	var cores = runtime.NumCPU()
	var tests = []struct {
		name        string
		nGoroutines int
		nEnqueue    int
	}{
		{
			name:        "100 Enqueue x 1 Goroutines",
			nGoroutines: 1,
			nEnqueue:    100,
		},
		{
			name:        "1 Enqueue x 100 Goroutines",
			nGoroutines: 100,
			nEnqueue:    1,
		},
		{
			name:        "100 Enqueue x CPU Cores Goroutines",
			nGoroutines: cores,
			nEnqueue:    100,
		},
		{
			name:        "100 Enqueue x (CPU Core x 5 Goroutines)",
			nGoroutines: cores * 5,
			nEnqueue:    100,
		},
	}

	var queue = NewAsyncDoubleQueue(10, logger)
	go queue.RunEventLoop()
	defer queue.Close()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			var total = test.nEnqueue * test.nGoroutines
			var wg sync.WaitGroup
			wg.Add(total)

			var value = 0
			var task = func() {
				value++
				wg.Done()
			}

			for j := 0; j < test.nGoroutines; j++ {
				go func() {
					for i := 0; i < test.nEnqueue; i++ {
						queue.Enqueue(task)
					}
				}()
			}

			wg.Wait()
			if value != total {
				t.Fatalf("value = %d, expected %d", value, total)
			}
		})
	}
}

func TestAsyncDoubleQueue_SlowConsumeFastProduce(t *testing.T) {

	var queue = NewAsyncDoubleQueue(10, logger)
	go queue.RunEventLoop()

	const totalTask = 10
	const total = 4

	var value = 0
	var task = func() {
		value++
	}

	for j := 0; j < total; j++ {
		for i := 0; i < totalTask; i++ {
			queue.Enqueue(task)
		}
		queue.Enqueue(func() {
			time.Sleep(500 * time.Millisecond)
		})
	}

	var wait = queue.Close()
	wait()

	if value != total*totalTask {
		t.Fatalf("value = %d, expected %d", value, total*totalTask)
	}
}
