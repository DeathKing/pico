package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

func isWildcardPath(path string) bool {
	return strings.Contains(path, "*") || strings.Contains(path, "?")
}

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

	usage = "output folder"
	flag.StringVar(&outputFolder, "o", ".", usage)
	flag.StringVar(&outputFolder, "output", ".", usage)

	usage = "output format (jpeg, png)"
	flag.StringVar(&outputFormat, "fmt", "jpeg", usage)
	flag.StringVar(&outputFormat, "format", "jpeg", usage)

	flag.Parse()

	if flag.NArg() == 0 {
		flag.PrintDefaults()
		os.Exit(0)
	}

	// there're files to process
	if flag.NArg() == 1 {
		if pdf := flag.Arg(0); !isWildcardPath(pdf) {
			fileInfo, err := os.Stat(pdf)
			if err != nil {
				fmt.Errorf("%v", err)
				os.Exit(1)
			}
			if !fileInfo.IsDir() {
				convertSingle(ctx, pdf)
				return
			}
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
	// go func() {
	task.Wait()
	for _, err := range task.Errors() {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	fmt.Println("task done")
	// }()
	bar.Wait()
	// go bar.Wait()
}

func convertBatch(ctx context.Context, pdfs []string) {
	task, _ := ConvertFiles(FromDirWalk(pdfs[0]),
		WithDpi(dpi),
		WithFormat(outputFormat),
		WithContext(ctx),
		WithOutputFileFn(func(pdf string, index, first, last int32) string {
			fileName := filepath.Base(pdf)
			return fileName[:len(fileName)-len(filepath.Ext(fileName))]
		}),
		WithWorkerCount(worker),
		WithOutputFolder(outputFolder))

	// assert.NoError(t, err, "conversion task initialization should not failed")

	bar := Bar(task)
	// go func() {
	task.Wait()
	for _, err := range task.Errors() {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	fmt.Println("task done")
	// }()
	bar.Wait()
	// go bar.Wait()
}

func main1() {
	total := 0
	wg := &sync.WaitGroup{}

	woker := func(id int, jobs <-chan string, pages chan<- int) {
		defer wg.Done()
		for file := range jobs {
			page, _ := GetPagesCount(file, WithTimeout(time.Second*5))
			// assert.NoErrorf(t, err, "GetPagesCount for file %s failed", file)
			fmt.Printf("%s: %d\n", file, page)
			pages <- page
		}
	}

	dir := "/Volumes/Elements/CLE-Corpus文献数据-已处理/"
	infos, _ := ioutil.ReadDir(dir)

	jobs := make(chan string, len(infos))
	pages := make(chan int, len(infos))

	go func() {
		defer close(jobs)
		err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".pdf") {
				jobs <- path
			}
			return nil
		})
		if err != nil {
			fmt.Printf("filepath.Walk failed: %v\n", err)
		}
	}()

	const numJobs = 4
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go woker(i, jobs, pages)
	}

	go func() {
		wg.Wait()
		close(pages)
	}()

	for p := range pages {
		total += p
	}

	fmt.Printf("total pages: %d", total)
}
