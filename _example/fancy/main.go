package main

import (
	"fmt"
	"time"

	"github.com/DeathKing/pico"
)

func main() {
	task, _ := pico.Convert("./tests/test_241.pdf",
		pico.WithOutputFolder("./out"),
		pico.WithFormat("jpg"),
		pico.WithDpi(72),
		pico.WithPageRange(22, 42),       // Convert from Page 22 to Page 42 (included)
		pico.WithJob(3),                  // Using 3 worker/process to convert
		pico.WithTimeout(10*time.Second), // Must finished within 10 seconds
	)

	for _, entry := range task.WaitAndCollect() {
		fmt.Printf("[worker#%s] file: %s %s/%s\n", entry[3], entry[2], entry[0], entry[1])
	}
}
