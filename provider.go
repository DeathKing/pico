package gopdf2image

import (
	"io/ioutil"
	"path/filepath"
)

type FileProvider interface {
	Source() <-chan string
}

type BaseProvider struct {
	source chan string
}

type SliceFileProvider struct {
	BaseProvider
}

func (p *BaseProvider) Source() <-chan string {
	return p.source
}

func FromSlice(files []string) FileProvider {
	source := make(chan string, len(files))
	for _, file := range files {
		source <- file
	}
	close(source)
	return &SliceFileProvider{BaseProvider{source}}
}

func FromDir(dir string) FileProvider {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	files := []string{}
	for _, file := range infos {
		if !file.IsDir() {
			files = append(files, filepath.Join(dir, file.Name()))
		}
	}

	return FromSlice(files)
}

func FromChan(ch chan string) FileProvider {
	return &BaseProvider{source: ch}
}
