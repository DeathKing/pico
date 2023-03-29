package main

import (
	"fmt"
	"math/rand"
	"time"
	"unicode/utf8"

	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"

	. "github.com/DeathKing/pico"
)

// ws means window size
func Marquee(textGetter func() string, ws uint, wcc ...decor.WC) decor.Decorator {
	var count uint
	buf := make([]byte, ws+1)
	f := func(s decor.Statistics) string {
		text := textGetter()
		bytes := []byte(text)
		start := count % uint(len([]rune(text)))

		var i uint = 0
		var ri uint = 0
		for pos, r := range text {
			if ri < start {
				ri++
				continue
			}
			l := uint(utf8.RuneLen(r))
			if i+l > ws {
				break
			}
			copy(buf[i:i+l], bytes[pos:uint(pos)+l])
			i += l
		}
		for ; i <= ws; i++ {
			buf[i] = ' '
		}
		count++
		return string(buf)
	}
	return decor.Any(f, wcc...)
}

func Bar(task interface{}) *mpb.Progress {
	switch t := task.(type) {
	case *SingleTask:
		return singleTaskBar(t)
	case *BatchTask:
		return batchTaskBar(t)
	default:
		panic("unknown task type")
	}
}

// the semantics of Bar() between SingleDocTask and BatchDocTask is different
func batchTaskBar(t *BatchTask) *mpb.Progress {
	p := mpb.New()

	for id, convertor := range t.Convertors {
		worker := fmt.Sprintf("Worker#%02d:", id)

		c := convertor
		status := Marquee(func() string {
			return c.Filename()
		}, 30)
		status = decor.OnComplete(status, "\x1b[32mdone!\x1b[0m")
		status = decor.OnAbort(status, "\x1b[31maborted\x1b[0m")

		// wc := status.GetConf()
		// wc.FormatMsg(convertor.pdf)

		bar := p.AddBar(int64(convertor.Total()),
			mpb.PrependDecorators(
				decor.Name(worker, decor.WC{W: len(worker) + 1, C: decor.DidentRight}),
				status,
				decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(decor.Percentage(decor.WC{W: 5})),
		)

		go completeWorker(bar, convertor, status.GetConf())
	}

	return p
}

func completeWorker(bar *mpb.Bar, c *Convertor, wc decor.WC) {
	for !bar.Completed() {
		converted, total := c.Progress()
		if c.Completed() {
			bar.SetTotal(int64(total), true)
		} else {
			bar.SetTotal(int64(total), false)
			bar.SetCurrent(int64(converted))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// the semantics of Bar() between SingleDocTask and BatchDocTask is different
func singleTaskBar(t *SingleTask) *mpb.Progress {
	p := mpb.New()

	for id, convertor := range t.Convertors {
		worker := fmt.Sprintf("Worker#%02d:", id)

		c := convertor
		status := Marquee(func() string {
			return c.Filename()
		}, 30)
		status = decor.OnComplete(status, "\x1b[32mdone!\x1b[0m")
		status = decor.OnAbort(status, "\x1b[31maborted\x1b[0m")

		bar := p.AddBar(int64(convertor.Total()),
			mpb.PrependDecorators(
				decor.Name(worker, decor.WC{W: len(worker) + 1, C: decor.DidentRight}),
				status,
				decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(decor.Percentage(decor.WC{W: 5})),
		)

		go complete(bar, convertor)
	}

	return p
}

func complete(bar *mpb.Bar, c *Convertor) {
	max := 500 * time.Millisecond

	for !bar.Completed() {
		time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)
		if c.Abroted {
			bar.Abort(false)
		} else {
			conveted, _ := c.Progress()
			bar.SetCurrent(int64(conveted))
		}
	}
}
