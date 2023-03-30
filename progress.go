package pico

import (
	"sync/atomic"
)

type Observable interface {
	// Total is the total
	Total() int32

	// Finished counts finished conversion, since the conversion may be a
	// part of a file, like from `firstPage` to `lastPage`, thus the total
	// count may less than `lastPage` and Finished() <= Current() always holds
	Finished() int32

	// Current is the current page number we've just converted
	Current() int32

	Completed() bool
	Aborted() bool
}

type Progress struct {
	// pdf is the full path of the pdf file
	pdf string

	// current is the page number currently be converted
	current int32

	total    int32
	finished int32
}

func (p *Progress) Filename() string {
	return p.pdf
}

func (p *Progress) Total() int32 {
	return atomic.LoadInt32(&p.total)
}

func (p *Progress) Finished() int32 {
	return atomic.LoadInt32(&p.finished)
}

func (p *Progress) Incr(delta int32) {
	atomic.AddInt32(&p.finished, delta)
}

func (p *Progress) PushTotal(delta int32) {
	atomic.AddInt32(&p.total, delta)
}

func (p *Progress) SetCurrent(current int32) {
	atomic.StoreInt32(&p.current, current)
}

func (p *Progress) Current() int32 {
	return atomic.LoadInt32(&p.current)
}

func (p *Progress) setInit(pdf string, first, last int32) {
	p.pdf = pdf

	atomic.StoreInt32(&p.current, first)
	atomic.StoreInt32(&p.total, last-first+1)
	atomic.StoreInt32(&p.finished, 0)
}

func (p *Progress) setWaiting() {
	p.pdf = "<waiting>"

	atomic.StoreInt32(&p.current, -1)
}
