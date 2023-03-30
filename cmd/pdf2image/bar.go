package main

import (
	"fmt"
	"time"

	"github.com/DeathKing/pico"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// Marquee is useful when displaying long text
func Marquee(textGetter func() string, ws uint, wcc ...decor.WC) decor.Decorator {
	var count uint
	f := func(s decor.Statistics) string {
		text := textGetter()
		runes := []rune(text)

		msg := string(runes[int(count)%len(runes):])
		count++

		return runewidth.FillRight(
			runewidth.Truncate(msg, int(ws), ""),
			int(ws))
	}
	return decor.Any(f, wcc...)
}

func Bar(task interface{}) *mpb.Progress {
	switch t := task.(type) {
	case *pico.SingleTask:
		return singleTaskBar(t)
	case *pico.BatchTask:
		return batchTaskBar(t)
	default:
		panic("unknown task type")
	}
}

var _txtDone = "\x1b[32mDone!\x1b[0m"
var _txtAbort = "\x1b[31mAborted\x1b[0m"

func singleTaskBar(t *pico.SingleTask) *mpb.Progress {
	p := mpb.New()

	for id, convertor := range t.Convertors {
		worker := fmt.Sprintf("Worker#%02d:", id)

		c := convertor
		status := Marquee(func() string {
			return c.Filename()
		}, 30)
		status = decor.OnComplete(status, _txtDone)
		status = decor.OnAbort(status, _txtAbort)

		bar := p.AddBar(0,
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

func batchTaskBar(t *pico.BatchTask) *mpb.Progress {
	p := mpb.New()

	// total file count
	name := "Total file"
	bar := p.AddBar(0,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
			decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(decor.Percentage(decor.WC{W: 5})),
	)
	go complete(bar, t)

	for id, convertor := range t.Convertors {
		worker := fmt.Sprintf("Worker#%02d:", id)

		c := convertor
		status := Marquee(func() string {
			return c.Filename()
		}, 30)
		status = decor.OnComplete(status, _txtDone)
		status = decor.OnAbort(status, _txtAbort)

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

func complete(bar *mpb.Bar, o pico.Observable) {
	for !bar.Completed() {
		time.Sleep(time.Duration(500))
		switch {
		case o.Aborted():
			bar.Abort(false)
		case o.Completed():
			bar.SetTotal(int64(o.Total()), true)
		default:
			bar.SetTotal(int64(o.Total()), false)
			bar.SetCurrent(int64(o.Finished()))
		}
	}
}
