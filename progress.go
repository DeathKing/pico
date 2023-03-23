package pico

import (
	"sync/atomic"
)

type progress struct {
	// pdf is the full path of the pdf file
	pdf string

	// current is the page number currently be converted
	current   int32
	total     int32
	converted int32
}

func (p *progress) Filename() string {
	return p.pdf
}

func (p *progress) Total() int32 {
	return atomic.LoadInt32(&p.total)
}

func (p *progress) Incr(delta int32) {
	atomic.AddInt32(&p.converted, delta)
}

func (p *progress) PushTotal(delta int32) {
	atomic.AddInt32(&p.total, delta)
}

func (p *progress) Progress() (int32, int32) {
	total := atomic.LoadInt32(&p.total)
	converted := atomic.LoadInt32(&p.converted)
	return converted, total
}

func (p *progress) SetCurrent(current int32) {
	atomic.StoreInt32(&p.current, current)
}

func (p *progress) Current() int32 {
	return atomic.LoadInt32(&p.current)
}

func (p *progress) setInit(pdf string, first, last int32) {
	p.pdf = pdf

	atomic.StoreInt32(&p.current, first)
	atomic.StoreInt32(&p.total, last-first+1)
	atomic.StoreInt32(&p.converted, 0)
}

func (p *progress) setWaiting() {
	p.pdf = "<waiting>"

	atomic.StoreInt32(&p.current, -1)
}
