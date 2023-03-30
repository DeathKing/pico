package main

import "github.com/DeathKing/pico"

func main() {
	task, _ := pico.Convert("tests/test_241.pdf",
		pico.WithOutputFolder("./out"),
	)
	task.Wait()
}
