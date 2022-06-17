package gopdf2image

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

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

	for id, convertor := range t.convertors {
		worker := fmt.Sprintf("Worker#%02d:", id)

		status := decor.Name("converting", decor.WCSyncSpaceR)
		status = decor.OnComplete(status, "\x1b[32mdone!\x1b[0m")
		status = decor.OnAbort(status, "\x1b[31maborted\x1b[0m")

		// wc := status.GetConf()
		// wc.FormatMsg(convertor.pdf)

		bar := p.AddBar(int64(atomic.LoadInt32(&convertor.total)),
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

func completeWorker(bar *mpb.Bar, c *convertor, wc decor.WC) {
	// max := 50 * time.Millisecond

	for !bar.Completed() {
		if c.completed() {
			bar.SetTotal(int64(c.total), true)
		} else {
			// wc.FormatMsg(c.pdf)
			bar.SetTotal(int64(c.total), false)
			bar.SetCurrent(int64(c.converted))
		}
		// time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)
		time.Sleep(50 * time.Millisecond)
	}
}

// the semantics of Bar() between SingleDocTask and BatchDocTask is different
func singleTaskBar(t *SingleTask) *mpb.Progress {
	p := mpb.New()

	for id, convertor := range t.convertors {
		worker := fmt.Sprintf("Worker#%02d:", id)

		status := decor.Name("converting", decor.WCSyncSpaceR)
		status = decor.OnComplete(status, "\x1b[32mdone!\x1b[0m")
		status = decor.OnAbort(status, "\x1b[31maborted\x1b[0m")

		bar := p.AddBar(int64(convertor.total),
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

func complete(bar *mpb.Bar, c *convertor) {
	max := 500 * time.Millisecond

	for !bar.Completed() {
		time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)
		// if c.Error != nil {
		if c.abroted {
			bar.Abort(false)
		} else {
			bar.SetCurrent(int64(c.converted))
		}
	}
}
