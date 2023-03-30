package main

import "github.com/DeathKing/pico"

func main() {
	task, _ := pico.ConvertFiles(
		pico.FromSlice([]string{
			"./tests/test_14.pdf",
			"./tests/test_241.pdf",
		}),
	)

	task.Wait()
}
