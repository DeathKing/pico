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

func (p *progress) PushTotal(delta int32) {
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

type Task struct {
	progress

	err error
	wg  *sync.WaitGroup

	// Params is the final computed arguments used to invoke the conversion call
	Params *Parameters

	Convertors []*konvertor

	// SubTasks will be useful when you want to invistgate worker-specific progress
	SubTasks []*ConversionSubTask

	// Entries is the channel of conversion progress entry
	// the format will be ["currentPage" "lastPage" "filename" "workerID"]
	Entries chan []string

	// Done
	Done chan interface{}
}

// buildConvertor creates one time convertor for single PDF file convertion. It
// first computes page ranges each convertor should take care of, then computes
// output folder and filename, then create a subprocess used to actually perfrom
// that conversion.
func (t *Task) buildConvertor(nth int32, base []string) {
	t.wg.Add(1)
	p := t.Params

	// 1. page calculation
	minPagePerConvertor := p.pageCount / p.workerCount
	reminder := p.pageCount % p.workerCount

	amortization := int32(0)
	if nth < reminder {
		amortization = 1
	}

	firstPage := p.firstPage + nth*minPagePerConvertor
	lastPage := firstPage + reminder + minPagePerConvertor - 1 + amortization

	if lastPage > p.lastPage {
		panic("Wrong calculation, worker lastPage should not greater than task lastPage")
	}

	// 2. Filename and outputfolder calculation
	outputFile := p.outputFile
	if p.outputFileFn != nil {
		outputFile = p.outputFileFn(p.pdfPath, nth, firstPage, lastPage)
	}

	outputFolder := p.outputFolder
	if p.outputFolderFn != nil {
		outputFolder = p.outputFolderFn(p.pdfPath, nth, firstPage, lastPage)
	}

	outputFile = filepath.Join(outputFolder, outputFile)

	command := []string{
		getCommandPath(p.binary, p.popplerPath),
		"-f", strconv.Itoa(int(firstPage)),
		"-l", strconv.Itoa(int(lastPage)),
	}
	command = append(command, base...)
	command = append(command, p.pdfPath, outputFile)

	// create subprocess

	worker := &konvertor{}

	t.Convertors = append(t.Convertors, worker)
}

func (t *Task) buildConvertWorker(nth int32, base []string) {

}

type konvertor struct {
	progress
	cmd  *exec.Cmd
	errs []error

	fisrt int32
	last  int32

	Params *Parameters
	ID     int32
}

type reusableKonvertor struct {
	konvertor

	FileProvider
}

func createCovertor(nth int, p *Parameters, command []string) {

}

func (rk *reusableKonvertor) Start() {
	source := rk.Source()
	// FIXME: preventing multiple start

	for {
		// if we doesn't have a file, we need to get one
		if rk.cmd == nil {
			// accquire one
			select {
			case <-rk.Params.ctx.Done():
				// global timeout
				return
			case file, more := <-source:
				if !more {
					return
				}
				// cmd := f(file)
			}
		}

		select {
		// this is global control
		case <-rk.Params.ctx.Done():
			return
		case <-time.After(rk.Params.perPageTimeout):
			return
		case entry, more := <-rk.Entries:
			// release rk.cmd
			if !more {
				rk.cmd = nil
			} else {
				// propergate to parent
			}
			// processing file

		}
	}
}

type SingleConvertor struct {
	Task
}

type BatchConvertor struct {
	Task
}

type ConversionSubTask struct {
	progress
	cmd        *exec.Cmd
	stderrPipe io.ReadCloser

	ID     int32
	Parent *Task
	Error  error
}

type worker struct {
	progress
	cmd *exec.Cmd

	ID     int32
	Parent *Task
	Error  error
}

func newConversionSubTask(id int32, task *Task) *ConversionSubTask {
	return &ConversionSubTask{
		ID:     id,
		Parent: task,
	}
}

func newConversion(params *Parameters, pageCount int32) *Task {
	chansize := int32(0)
	if params.progress {
		chansize = pageCount
	}

	return &Task{
		wg: &sync.WaitGroup{},

		Params:  params,
		Entries: make(chan []string, chansize),
		Done:    make(chan interface{}),
	}
}

func (c *Task) Errors() []error {
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

func (c *Task) Error() error {
	if errs := c.Errors(); len(errs) > 0 {
		return errs[0]
	}

	return c.err
}

// Start initiates the conversion process
func (c *Task) Start() error {
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
		c.Params.cancel()
	}()

	return nil
}

// Wait hijacks the EntryChan and wait for all the workers finish
func (c *Task) Wait() {
	for range c.Entries {
	}
}

func (c *Task) WaitAndErrors() []error {
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

func (c *Task) Bar() *Task {
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

func (c *Task) WaitWithBar() {
	c.Bar().Wait()
}

// WaitAndCollect acts like Wait() but collects all the entries into a slice.
// A empty array is returned if there is no entry received.
func (c *Task) WaitAndCollect() (entries [][]string) {
	entries = make([][]string, 0)
	for entry := range c.Entries {
		entries = append(entries, entry)
	}
	return entries
}

func (c *Task) onEntry(entry []string) {
	if c.Params.progress {
		c.Entries <- entry
	}
}

func (c *Task) onWokerComplete() {

}

func (c *Task) createWorker(nth int32, base []string) *ConversionSubTask {
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
		defer cst.Parent.wg.Done()
		defer cst.cmd.Wait()

		call := cst.Parent.Params
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
				cst.Parent.Entries <- item
				cst.Parent.Incr(1)
			}
		}
	}
}

func (cst *ConversionSubTask) useProgressReader() (chan []string, func()) {
	id := strconv.Itoa(int(cst.ID))
	call := cst.Parent.Params
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
						NewOldPDFSyntaxError(line, call.pdfPath, cst.Current),
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

func (cst *ConversionSubTask) setError(err error) {
	if cst.Error == nil {
		cst.Error = err
	}
}

type reader struct {
	onError    func(err error) bool
	onEntry    func(entries []string)
	onComplete func()
}

type SilentReader struct {
	reader
}

type ProgressReader struct {
	reader
}

func (pr *ProgressReader) ReadLoop(source io.ReadCloser) {
	defer source.Close() // FIXME: should we close it?
	defer pr.onComplete()

	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "Syntax Error") {
			err := errors.WithStack(NewPDFSyntaxError(line))
			if kontinue := pr.onError(err); !kontinue {
				return
			}
		}

		if entry := strings.Split(line, " "); len(entry) == 3 {
			pr.onEntry(entry)
		}
	}
}

type ProgressController struct {
	cst     *ConversionSubTask
	entries chan []string
}

func (pc *ProgressController) onEntry(entry []string) {
	if current, err := strconv.Atoi(entry[0]); err == nil {
		pc.entries <- entry
		pc.cst.Current = int32(current)
		pc.cst.Incr(1)
	}
}

func (pc *ProgressController) onError(err error) bool {
	if !pc.cst.Parent.Params.strict {
		return true
	}
	pc.cst.Error = errors.Wrap(err, "progressController:")
	return false
}

func (pc *ProgressController) onComplete() {
	close(pc.entries)
}

func (pc *ProgressController) ControlLoop() {
	defer pc.cst.Parent.wg.Done()
	defer pc.cst.cmd.Wait()

	call := pc.cst.Parent.Params
	hint := fmt.Sprintf("worker#%d progressController:", pc.cst.ID)

	for {
		select {
		case <-call.ctx.Done():
			pc.cst.setError(errors.Wrap(call.ctx.Err(), hint))
			return
		case <-time.After(call.perPageTimeout):
			pc.cst.setError(errors.Wrap(
				NewPerPageTimeoutError(pc.cst.Current),
				hint,
			))
			return
		case item, more := <-pc.entries:
			if !more {
				return
			}
			pc.cst.Parent.Entries <- item
			pc.cst.Parent.Incr(1)
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
		defer cst.Parent.Incr(cst.Total)

		call := cst.Parent.Params
		scanner := bufio.NewScanner(cst.stderrPipe)

		for scanner.Scan() {
			line := scanner.Text()

			// should we continue other worker when error happens?
			if call.strict {
				if strings.Contains(line, "Syntax Error") {
					cst.Error = errors.Wrapf(
						NewOldPDFSyntaxError(line, call.pdfPath, -1),
						"worker#%d silentReader:", cst.ID)
					return
				}
			}
		}
	}
}

func (cst *ConversionSubTask) useSilentController(done chan interface{}) func() {
	return func() {
		defer cst.Parent.wg.Done()
		defer cst.cmd.Wait()

		call := cst.Parent.Params
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
