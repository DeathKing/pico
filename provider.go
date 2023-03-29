package pico

import (
	"fmt"
	"os"
	"path/filepath"
)

const _dirwalkchansize = 100

type PdfProvider interface {
	Source() <-chan string
	Count() int
}

type ChanProvider struct {
	source chan string
}

type SliceFileProvider struct {
	ChanProvider

	len int
}

func (p *ChanProvider) Source() <-chan string {
	return p.source
}

func (p *ChanProvider) Count() int {
	return -1
}

func FromSlice(files []string) PdfProvider {
	source := make(chan string, len(files))
	defer close(source)
	for _, file := range files {
		source <- file
	}
	return &SliceFileProvider{
		ChanProvider{source},
		len(files),
	}
}

func (p *SliceFileProvider) Count() int {
	return p.len
}

func FromGlob(pattern string) PdfProvider {
	files, _ := filepath.Glob(pattern)

	return FromSlice(files)
}

func FromChan(ch chan string) PdfProvider {
	return &ChanProvider{source: ch}
}

func FromMultiSource(patterns []string) PdfProvider {
	files := []string{}
	for _, pattern := range patterns {
		info, _ := os.Stat(pattern)

		if info.IsDir() {
			pattern = filepath.Join(pattern, "/*.pdf")
		}

		batch, _ := filepath.Glob(pattern)
		files = append(files, batch...)
	}

	fmt.Printf("%v\n", len(files))
	return FromSlice(files)
}

func FromMultiSourceAsync(patterns []string) PdfProvider {
	ch := make(chan string, _dirwalkchansize)
	go func() {
		defer close(ch)
		for _, pattern := range patterns {
			info, _ := os.Stat(pattern)

			if info.IsDir() {
				pattern = filepath.Join(pattern, "/*")
			}

			batch, _ := filepath.Glob(pattern)
			for _, file := range batch {
				ch <- file
			}
		}
	}()

	return FromChan(ch)
}

func FromInterface(i interface{}) PdfProvider {
	switch i := i.(type) {
	case PdfProvider:
		return i
	case string:
		return FromGlob(i)
	case []string:
		return FromSlice(i)
	case chan string:
		return FromChan(i)
	}
	panic("unsupported type")
}
