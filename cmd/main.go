package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	pdf2image "github.com/DeathKing/go-pdf2image"
)

const numWorkers = 4

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	dir := "D:/from"

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		fmt.Println("Processing: ", path)
		if err != nil {
			fmt.Println(err)
			return err
		}
		// fmt.Printf("dir: %v: name: %s\n", info.IsDir(), path)
		if info.IsDir() {
			return nil
		}

		task, err := convert(path)
		if err != nil {
			return err
		}

		task.Bar().Wait()

		for _, err := range task.Errors() {
			fmt.Printf("%+v\n", err)
		}

		if len(task.Errors()) > 0 {
			os.Exit(1)
		}

		base := filepath.Base(path)
		os.Rename(path, fmt.Sprintf("D:/processed/%s", base))

		return nil
	})
	if err != nil {
		fmt.Println(err)
	}

}

func convert(file string) (*pdf2image.Task, error) {
	return pdf2image.Convert(
		file,
		pdf2image.WithWorkerCount(numWorkers),
		// pdf2image.WithTimeout(time.Second*100),
		pdf2image.WithStrict(),
		pdf2image.WithFormat("jpg"),
		pdf2image.WithProgress(),
		pdf2image.WithOutputFolder("D:/out"),
	)
}
