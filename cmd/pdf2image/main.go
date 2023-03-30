package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/DeathKing/pico"
)

// usage:
// pdf2image path-to-pdf-file
// -j | --job
// -wid | --worker-id
//     append worker id when output file
// -d | --dpi
// -f | --first-page
// -l | --last-page
// -fmt | --format
// -upw | --user-password
// -opw | --oener-password
// -t | --timeout
// -opt | --optimize
// -o | --output-folder
//    set output folder name
// --slient
//    do not display any infomation
// --entry
// --bar (default)
//    show conversion progress bar
// -gray | --grayscale
// -trans | --transparent

var (
	dpi          int
	worker       int
	firstPage    int
	lastPage     int
	outputFolder string
	outputFormat string

	appendWorkerId bool

	nameFn = func(pdf string, index, first, last int32) string {
		wid := ""
		if appendWorkerId {
			wid = strconv.Itoa(int(index))
		}

		fileName := filepath.Base(pdf)
		return fmt.Sprintf("%s-%s", fileName[:len(fileName)-len(filepath.Ext(fileName))], wid)
	}
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

	usage = "append worker id in filename"
	flag.BoolVar(&appendWorkerId, "wid", false, usage)
	flag.BoolVar(&appendWorkerId, "worker-id", false, usage)

	usage = "fisrt page"
	flag.IntVar(&firstPage, "f", -1, usage)
	flag.IntVar(&firstPage, "first-page", -1, usage)

	usage = "last page"
	flag.IntVar(&lastPage, "l", -1, usage)
	flag.IntVar(&lastPage, "last-page", -1, usage)

	usage = "output folder"
	flag.StringVar(&outputFolder, "o", ".", usage)
	flag.StringVar(&outputFolder, "output-folder", ".", usage)

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
	task, _ := pico.Convert(pdf,
		pico.WithDpi(dpi),
		pico.WithFormat(outputFormat),
		pico.WithContext(ctx),
		pico.WithOutputFileFn(nameFn),
		pico.WithJob(worker),
		pico.WithOutputFolder(outputFolder))

	bar := Bar(task)

	task.Wait()
	for _, err := range task.Errors() {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	bar.Wait()
}

func convertBatch(ctx context.Context, pdfs []string) {
	task, _ := pico.ConvertFiles(pico.FromMultiSource(pdfs),
		pico.WithDpi(dpi),
		pico.WithFormat(outputFormat),
		pico.WithContext(ctx),
		pico.WithOutputFileFn(nameFn),
		pico.WithFirstPage(firstPage),
		pico.WithLastPage(lastPage),
		pico.WithJob(worker),
		pico.WithOutputFolder(outputFolder))

	bar := Bar(task)

	task.Wait()
	bar.Wait()

	for _, err := range task.Errors() {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
}
