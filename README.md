# AsyncQ

![Go Report Card](https://goreportcard.com/badge/github.com/ggpslop/asyncq) [![Go Doc](https://godoc.org/github.com/ggpslop/asyncq?status.svg)](https://pkg.go.dev/github.com/ggpslop/asyncq)

#### Simple and Fast Asynchronous Queue and Event Loop in GO.

### Implementations

The package exposes an interface **AsyncQ** and one implementation **AsyncDoubleQueue**.

**AsyncDoubleQueue** uses two queues, one in for enqueue operations (*inputQueue*) and another one for dequeues operations (*outputQueue*).
  - A single mutex is used to lock only the *inputQueue*. The event loop acquire the lock only in one occasion: when *outputQueue* is empty and exchanges the two queues;
  - Use a channel to put the event loop to sleep or to wake-up it;
  - The event loop is **panic safe**;
  - A **nil task** closes the event loop (but not the channel, so it's safe to only use **Close**).

### Example

```go
package main

import (
    "fmt"
    "log"
    "os"
	
    "github.com/ggpslop/asyncq"
)

func main() {
    var queue AsyncQ
    queue = NewAsyncDoubleQueue(10, log.New(os.Stdout, "", 0))
    
    go queue.RunEventLoop()
    defer queue.Close()
    
    word := "world"
    queue.Enqueue(func() {
      fmt.Printf("Hello, %s!\n", word)
    })
}
```