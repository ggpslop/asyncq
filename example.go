package asyncq

import (
	"fmt"
	"log"
	"os"
)

func example() {

	var queue AsyncQ
	queue = NewAsyncDoubleQueue(10, log.New(os.Stdout, "", 0))

	go queue.RunEventLoop()
	defer queue.Close()

	word := "world"
	queue.Enqueue(func() {
		fmt.Printf("Hello, %s!\n", word)
	})
}
