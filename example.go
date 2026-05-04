//go:build ignore

package asyncq

import (
	"fmt"
	"log"
	"os"
)

func example_deferred_close() {

	var queue AsyncQ
	queue = NewAsyncDoubleQueue(10, log.New(os.Stdout, "", 0))

	go queue.RunEventLoop()
	defer queue.Close()

	var word = "world"
	queue.Enqueue(func() {
		fmt.Printf("Hello, %s!\n", word) // expected "Hello, world!"
	})
}

func example_close_plus_wait() {

	var queue AsyncQ
	queue = NewAsyncDoubleQueue(10, log.New(os.Stdout, "", 0))
	go queue.RunEventLoop()

	var number int // escapes to heap.
	queue.Enqueue(func() {
		number++
	})

	var wait = queue.Close()
	wait()

	fmt.Printf("The number is %d\n", number) // expected 2
}

func example_deferred_close_plus_wait() {

	var queue AsyncQ
	queue = NewAsyncDoubleQueue(10, log.New(os.Stdout, "", 0))
	go queue.RunEventLoop()

	defer func() {
		var wait = queue.Close()
		wait()
	}()

	var word = "world"
	queue.Enqueue(func() {
		fmt.Printf("Hello, %s!\n", word) // expected "Hello, world!"
	})
}
