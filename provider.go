package gopdf2image

import (
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
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

func FromDir(dir string) PdfProvider {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	files := []string{}
	for _, file := range infos {
		name := strings.ToLower(file.Name())
		if !file.IsDir() && strings.HasSuffix(name, ".pdf") {
			files = append(files, filepath.Join(dir, file.Name()))
		}
	}

	return FromSlice(files)
}

func FromDirWalk(dir string) PdfProvider {
	ch := make(chan string, _dirwalkchansize)
	go func() {
		defer close(ch)
		filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return errors.Wrapf(err, "filepath.Walk failed: %q", path)
			}
			name := strings.ToLower(info.Name())
			if !info.IsDir() && strings.HasSuffix(name, ".pdf") {
				ch <- path
			}
			return nil
		})
	}()
	return FromChan(ch)
}

func FromDirAsync(dir string) PdfProvider {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	files := make(chan string, len(infos))
	go func() {
		defer close(files)
		for _, file := range infos {
			if !file.IsDir() {
				files <- filepath.Join(dir, file.Name())
			}
		}
	}()

	return FromChan(files)
}

func FromChan(ch chan string) PdfProvider {
	return &ChanProvider{source: ch}
}

func FromInterface(i interface{}) PdfProvider {
	switch i := i.(type) {
	case PdfProvider:
		return i
	case string:
		return FromDir(i)
	case []string:
		return FromSlice(i)
	case chan string:
		return FromChan(i)
	}
	panic("unsupported type")
}
