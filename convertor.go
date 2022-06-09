package gopdf2image

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

type progress struct {
	Fisrt     int32
	Last      int32
	Current   int32
	Total     int32
	Converted int32
}

func (p *progress) Incr(delta int32) {
	atomic.AddInt32(&p.Converted, delta)
}

func (p *progress) PushNew(delta int32) {
	atomic.AddInt32(&p.Total, delta)
}

func (p *progress) Progress() (int32, int32) {
	return p.Converted, p.Total
}

// TODO just initialize using the struct
func (p *progress) setInit(first, last, current int32) {
	p.Fisrt = first
	p.Last = last
	p.Current = current
	p.Converted = 0
}

type Conversion struct {
	progress

	err error
	wg  *sync.WaitGroup

	// Params is the final computed arguments used to invoke the conversion call
	Params *Parameters

	// SubTasks will be useful when you want to invistgate worker-specific progress
	SubTasks []*ConversionSubTask

	// Entries is the channel of conversion progress entry
	// the format will be ["currentPage" "lastPage" "filename" "workerID"]
	Entries chan []string

	// Done
	Done chan interface{}
}

type ConversionSubTask struct {
	progress
	cmd        *exec.Cmd
	stderrPipe io.ReadCloser

	ID    int32
	Task  *Conversion
	Error error
}

func newConversionSubTask(id int32, task *Conversion) *ConversionSubTask {
	return &ConversionSubTask{
		ID:   id,
		Task: task,
	}
}

func newConversion(params *Parameters, pageCount int32) *Conversion {
	chansize := int32(0)
	if params.progress {
		chansize = pageCount
	}

	return &Conversion{
		wg: &sync.WaitGroup{},

		Params:  params,
		Entries: make(chan []string, chansize),
		Done:    make(chan interface{}),
	}
}

func (c *Conversion) Errors() []error {
	errs := []error{}

	if c.err != nil {
		errs = append(errs, c.err)
	}

	for _, cst := range c.SubTasks {
		if cst.Error != nil {
			errs = append(errs, cst.Error)
		}
	}
	return errs
}

func (c *Conversion) Error() error {
	if errs := c.Errors(); len(errs) > 0 {
		return errs[0]
	}

	return c.err
}

// Start initiates the conversion process
func (c *Conversion) Start() error {
	// FIXME: if any error occured during subtask initialization,
	// previous subtasks should be rewind.
	for _, cst := range c.SubTasks {
		if stderrPipe, err := cst.cmd.StderrPipe(); err != nil {
			return errors.WithStack(err)
		} else {
			cst.stderrPipe = stderrPipe
		}

		if err := cst.cmd.Start(); err != nil {
			return errors.WithStack(err)
		}

		c.wg.Add(1)

		if c.Params.progress {
			itemChan, progressReader := cst.useProgressReader()
			go cst.useProgressController(itemChan)()
			go progressReader()
		} else {
			done, silentReader := cst.useSlientReader()
			go cst.useSilentController(done)()
			go silentReader()
		}
	}

	go func() {
		c.wg.Wait()
		close(c.Entries)
		close(c.Done)
		c.Params.cancle()
	}()

	return nil
}

// Wait hijacks the EntryChan and wait for all the workers finish
func (c *Conversion) Wait() {
	for range c.Entries {
	}
}

func (c *Conversion) WaitAndErrors() []error {
	c.Wait()
	return c.Errors()
}

func complete(bar *mpb.Bar, subtask *ConversionSubTask) {
	max := 500 * time.Millisecond

	for !bar.Completed() {
		time.Sleep(time.Duration(rand.Intn(10)+1) * max / 10)
		if subtask.Error != nil {
			bar.Abort(false)
		} else {
			bar.SetCurrent(int64(subtask.Converted))
		}
	}
}

func (c *Conversion) Bar() *Conversion {
	p := mpb.New()

	for id := range c.SubTasks {
		subtask := c.SubTasks[id]
		worker := fmt.Sprintf("Worker#%02d:", id)

		status := decor.Name("converting", decor.WCSyncSpaceR)
		status = decor.OnComplete(status, "\x1b[32mdone!\x1b[0m")
		status = decor.OnAbort(status, "\x1b[31maborted\x1b[0m")

		bar := p.AddBar(int64(subtask.Total),
			mpb.PrependDecorators(
				decor.Name(worker, decor.WC{W: len(worker) + 1, C: decor.DidentRight}),
				status,
				decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(decor.Percentage(decor.WC{W: 5})),
		)

		go complete(bar, subtask)
	}

	p.Wait()

	return c
}

func (c *Conversion) WaitWithBar() {
	c.Bar().Wait()
}

// WaitAndCollect acts like Wait() but collects all the entries into a slice.
// A empty array is returned if there is no entry received.
func (c *Conversion) WaitAndCollect() (entries [][]string) {
	entries = make([][]string, 0)
	for entry := range c.Entries {
		entries = append(entries, entry)
	}
	return entries
}

func (c *Conversion) createWorker(nth int32, base []string) *ConversionSubTask {
	call := c.Params

	reminder := call.pageCount % call.workerCount
	amortization := int32(0)
	if nth < call.pageCount%call.workerCount {
		amortization = 1
	}

	firstPage := call.firstPage + nth*call.minPagePerWorker
	lastPage := firstPage + reminder + call.minPagePerWorker - 1 + amortization

	if lastPage > call.lastPage {
		lastPage = call.lastPage
	}

	// FIXME: outputFileFn support
	outputFolder := call.outputFolder
	outputFile := filepath.Join(outputFolder, call.outputFile)

	subtask := newConversionSubTask(nth, c)
	if call.progress {
		subtask.setInit(firstPage, lastPage, firstPage)
	}

	command := []string{
		getCommandPath(call.binary, call.popplerPath),
		"-f", strconv.Itoa(int(firstPage)),
		"-l", strconv.Itoa(int(lastPage)),
	}

	subtask.cmd = buildCmd(call.ctx, call.popplerPath,
		append(
			append(command, base...),
			call.pdfPath,
			outputFile,
		))

	if call.verbose {
		fmt.Printf("Worker#%02d: %s\n", nth, subtask.cmd.String())
	}

	return subtask
}

func (cst *ConversionSubTask) useProgressController(
	itemChan chan []string,
) func() {
	return func() {
		defer cst.Task.wg.Done()
		defer cst.cmd.Wait()

		call := cst.Task.Params
		hint := fmt.Sprintf("worker#%d progressController:", cst.ID)

		for {
			select {
			case <-call.ctx.Done():
				if cst.Error == nil {
					cst.Error = errors.Wrap(call.ctx.Err(), hint)
				}
				return
			case <-time.After(call.perPageTimeout):
				if cst.Error == nil {
					cst.Error = errors.Wrap(
						NewPerPageTimeoutError(cst.Current),
						hint,
					)
				}
				return
			case item, more := <-itemChan:
				if !more {
					return
				}
				cst.Task.Entries <- item
				cst.Task.Incr(1)
			}
		}
	}
}

func (cst *ConversionSubTask) useProgressReader() (chan []string, func()) {
	id := strconv.Itoa(int(cst.ID))
	call := cst.Task.Params
	itemChan := make(chan []string, call.minPagePerWorker+1)

	return itemChan, func() {
		defer close(itemChan)
		defer cst.stderrPipe.Close()
		scanner := bufio.NewScanner(cst.stderrPipe)

		for scanner.Scan() {
			line := scanner.Text()

			// should we continue other worker when error happens?
			if call.strict {
				if strings.Contains(line, "Syntax Error") {
					cst.Error = errors.Wrapf(
						NewPDFSyntaxError(line, call.pdfPath, cst.Current),
						"worker#%d progressReader:", cst.ID)
					return
				}
			}

			if tuples := strings.Split(line, " "); len(tuples) == 3 {
				tuples = append(tuples, id)
				if current, err := strconv.Atoi(tuples[0]); err == nil {
					itemChan <- tuples
					cst.Current = int32(current)
					cst.Incr(1)
				}
			}
		}
	}
}

func (cst *ConversionSubTask) useSlientReader() (chan interface{}, func()) {
	done := make(chan interface{})

	return done, func() {
		defer close(done)

		// When a subtask is done no matter if it's successful or not,
		// considered it has converted `cst.Total()` pages, because we cannot
		// know how many pages has been converted already without calling
		// poppler with `-progress` option.
		defer cst.Task.Incr(cst.Total)

		call := cst.Task.Params
		scanner := bufio.NewScanner(cst.stderrPipe)

		for scanner.Scan() {
			line := scanner.Text()

			// should we continue other worker when error happens?
			if call.strict {
				if strings.Contains(line, "Syntax Error") {
					cst.Error = errors.Wrapf(
						NewPDFSyntaxError(line, call.pdfPath, -1),
						"worker#%d silentReader:", cst.ID)
					return
				}
			}
		}
	}
}

func (cst *ConversionSubTask) useSilentController(done chan interface{}) func() {
	return func() {
		defer cst.Task.wg.Done()
		defer cst.cmd.Wait()

		call := cst.Task.Params
		hint := fmt.Sprintf("worker#%d silentController:", cst.ID)

		for {
			select {
			case <-call.ctx.Done():
				if cst.Error == nil {
					cst.Error = errors.Wrap(call.ctx.Err(), hint)
				}
				return
			case <-done:
				return
			}
		}
	}
}
