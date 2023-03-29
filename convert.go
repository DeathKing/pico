package pico

import (
	"github.com/pkg/errors"
)

// Convert converts single PDF to images. This function is solely a options parser
// and command builder
func Convert(pdf string, options ...CallOption) (*SingleTask, error) {
	p := defaultConvertCallOption()

	if err := p.apply(options...); err != nil {
		return nil, errors.WithStack(err)
	}

	// 1. page calculation
	pages, err := GetPagesCount(pdf, options...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	totalPage := int32(pages)

	if p.singleFile {
		p.firstPage = 1
		p.lastPage = 1
	}

	if p.lastPage < 0 || p.lastPage > totalPage {
		p.lastPage = totalPage
	}

	if p.firstPage > p.lastPage {
		return nil, errors.WithStack(newWrongPageRangeError(p.firstPage, p.lastPage))
	}

	// 2. worker number calculation
	p.pageCount = p.lastPage - p.firstPage + 1

	// workerCount is not set, we could infer for one
	if p.job <= 0 {
		switch {
		case p.pageCount > 50:
			p.job = 3
		case p.pageCount > 20:
			p.job = 2
		default:
			p.job = 1
		}
	}

	if p.job > p.pageCount {
		p.job = p.pageCount
	}

	p.minPagesPerWorker = p.pageCount / p.job

	task := newSingleTask(p)

	return task, task.Start(pdf)
}

// ConvertFiles converts multiple PDF files to images
//
// files could be type `[]string`, `chan string`, or `PdfProvider`
func ConvertFiles(files interface{}, options ...CallOption) (*BatchTask, error) {
	p := defaultConvertFilesCallOption()

	if err := p.apply(options...); err != nil {
		return nil, errors.WithStack(err)
	}

	provider := FromInterface(files)

	// automatically determine worker count, perfer using 4 worker
	p.job = determineWorkerCount(p.job, int32(provider.Count()))

	task := newBatchTask(p)

	return task, task.Start(provider)
}

func determineWorkerCount(set, need int32) int32 {
	switch {
	case set > 0:
		return set
	case need >= 20:
		return 4
	case need >= 10:
		return 2
	case need > 0:
		return 1
	default:
		return 4
	}
}
