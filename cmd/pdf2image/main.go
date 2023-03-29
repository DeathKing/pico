package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/DeathKing/pico"
)

// pico path-to-pdf-file
// -j | --job
// -d | --dpi
// -f | --first-page
// -l | --last-page
// -fmt | --format
// -upw | --user-password
// -opw | --oener-password
// -t | --timeout
// -opt | --optimize
// -o | --output
// --progressive
// --verbose
// -v | --version
// -gray | --grayscale
// -trans | --transparent

var (
	dpi          int
	worker       int
	firstPage    int
	lastPage     int
	outputFolder string
	outputFormat string
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	usage := "output dpi"
	flag.IntVar(&dpi, "d", 72, usage)
	flag.IntVar(&dpi, "dpi", 72, usage)

	usage = "worker count"
	flag.IntVar(&worker, "j", -1, usage)
	flag.IntVar(&worker, "job", -1, usage)

	usage = "fisrt page"
	flag.IntVar(&firstPage, "f", -1, usage)
	flag.IntVar(&firstPage, "first-page", -1, usage)

	usage = "last page"
	flag.IntVar(&lastPage, "l", -1, usage)
	flag.IntVar(&lastPage, "last-page", -1, usage)

	usage = "output folder"
	flag.StringVar(&outputFolder, "o", ".", usage)
	flag.StringVar(&outputFolder, "output", ".", usage)

	usage = "output format (jpeg, png)"
	flag.StringVar(&outputFormat, "fmt", "jpeg", usage)
	flag.StringVar(&outputFormat, "format", "jpeg", usage)

	flag.Parse()

	switch flag.NArg() {
	case 0:

		flag.PrintDefaults()
		os.Exit(0)
	case 1:
		pdf := flag.Arg(0)

		info, err := os.Stat(pdf)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		if !info.IsDir() {
			convertSingle(ctx, pdf)
			return
		}
	}

	convertBatch(ctx, flag.Args())

}

func convertSingle(ctx context.Context, pdf string) {
	task, _ := Convert(pdf,
		WithDpi(dpi),
		WithFormat(outputFormat),
		WithContext(ctx),
		WithOutputFileFn(func(pdf string, index, first, last int32) string {
			fileName := filepath.Base(pdf)
			return fileName[:len(fileName)-len(filepath.Ext(fileName))]
		}),
		WithWorkerCount(worker),
		WithOutputFolder(outputFolder))

	bar := Bar(task)

	task.Wait()
	for _, err := range task.Errors() {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	fmt.Println("task done")

	bar.Wait()
}

func convertBatch(ctx context.Context, pdfs []string) {
	task, _ := ConvertFiles(FromMultiSource(pdfs),
		WithDpi(dpi),
		WithFormat(outputFormat),
		WithContext(ctx),
		WithOutputFileFn(func(pdf string, index, first, last int32) string {
			fileName := filepath.Base(pdf)
			return fileName[:len(fileName)-len(filepath.Ext(fileName))]
		}),
		WithFirstPage(firstPage),
		WithLastPage(lastPage),
		WithWorkerCount(worker),
		WithOutputFolder(outputFolder))

	bar := Bar(task)

	task.Wait()
	for _, err := range task.Errors() {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}

	fmt.Println("task done")
	bar.Wait()
}
