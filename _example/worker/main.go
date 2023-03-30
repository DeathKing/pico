package main

import (
	"fmt"

	"github.com/DeathKing/pico"
)

func main() {
	task, _ := pico.Convert("./tests/test_241.pdf",
		pico.WithOutputFolder("./out"),
		pico.WithJob(4),
	)

	for entry := range task.Entries {
		fmt.Printf("page %s is converted as file %s \n",
			entry[0], // current page
			entry[2], // output filename
		)
	}
}
