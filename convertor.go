package gopdf2image

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/pkg/errors"
)

type progress struct {
	pdf string

	current   int32
	total     int32
	converted int32
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

func (p *progress) P() chan []int32 {
	ch := make(chan []int32, 1)
	ch <- []int32{p.total, p.converted}
	fmt.Printf("progress: %d %d\n", p.total, p.converted)
	return ch
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

type convertor struct {
	progress

	t   *Task
	id  int32
	cmd *exec.Cmd

	// converrs is a list of errors that occurred during the conversion.
	converrs []*ConversionError

	done chan interface{}

	abroted bool
}

// spwanCmdForPipe spwans an `exec.Cmd` for convererting the pdf from `first` to `last`,
//
func (c *convertor) spwanCmdForPipe(pdf string, first, last int32) (io.ReadCloser, error) {
	p := c.t.params
	command := p.buildCommand(pdf, c.id, first, last)
	c.cmd = buildCmd(p.ctx, p.popplerPath, command)

	// cmd.Wait() will close the pipe
	pipe, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err = c.cmd.Start(); err != nil {
		return nil, errors.WithStack(err)
	}

	return pipe, nil
}

func (c *convertor) Errors() []*ConversionError {
	return c.converrs
}

func (c *convertor) Error() (err error) {
	if len(c.converrs) > 0 {
		err = c.converrs[0].err
	}
	return
}

// receiveError 可能存在跨pdf error 的情况么？
func (c *convertor) receiveError(err error, page int32) bool {
	if err == nil {
		return true
	}

	c.converrs = append(c.converrs, &ConversionError{
		pdf:      c.pdf,
		page:     page,
		workerId: c.id,
		err:      err,
	})

	return !c.t.params.strict
}

// receiveEntry is called when an entry is received from the parser. This
// function is usually used to
//   1. update the progress
//   2. send the entry to the task
func (c *convertor) receiveEntry(entry []string) {
	current, _ := strconv.Atoi(entry[0])

	c.Incr(1)
	c.SetCurrent(int32(current))
	c.t.Entries <- entry
}

var _entryRE = regexp.MustCompile(`(\d+) (\d+) (.+)`)

func (c *convertor) parseProgress(pipe io.ReadCloser, ch chan<- []string, current int32) {
	scanner := bufio.NewScanner(pipe)
	defer close(ch)

	for scanner.Scan() {
		line := scanner.Text()

		// should we continue other worker when error happens?
		if strings.Contains(line, "Syntax Error") {
			err := errors.WithStack(NewPDFSyntaxError(line))
			if ok := c.receiveError(err, current); !ok {
				return
			}
		}

		// this is a critical error
		if strings.HasSuffix(line, "; exiting") {
			c.receiveError(errors.New(line), current)
			return
		}

		// if entry := strings.Split(line, " "); len(entry) == 3 {
		if entry := _entryRE.FindStringSubmatch(line); len(entry) > 3 {
			pg, _ := strconv.Atoi(entry[1])
			current = int32(pg)
			ch <- entry[1:]
		}
	}

	c.cmd.Wait()
}

// start starts the convertor
//
// errors may occur during spwan Cmd and pipe.
func (c *convertor) start(pdf string) error {
	p := c.t.params
	first, last, _ := p.pageRangeForPart(pdf, c.id)

	pipe, err := c.spwanCmdForPipe(pdf, first, last)
	if err != nil {
		return errors.WithStack(err)
	}

	c.progress.setInit(pdf, first, last)

	// ch is closed by `parseProgress`
	ch := make(chan []string, last-first+1)
	go c.parseProgress(pipe, ch, first)

	go func() {
		defer c.onComplete()
		for {
			select {
			case <-p.ctx.Done():
				c.receiveError(errors.WithStack(p.ctx.Err()), -1)
				return
			case entry, more := <-ch:
				if !more {
					return
				}
				c.receiveEntry(entry)
			}
		}
	}()

	return nil
}

func (c *convertor) startAsWorker(provider PdfProvider) {
	var pdf string
	var pipe io.ReadCloser
	var more bool
	var ch chan []string

	defer c.onComplete()

	p := c.t.params

	for {
		if c.cmd == nil {
			// accuquire a file for conversion
			select {
			case <-p.ctx.Done():
				c.receiveError(errors.WithStack(p.ctx.Err()), -1)
				return
			case pdf, more = <-provider.Source():
				if !more {
					return
				}
			}

			// page calculation, spwan cmd and pipe
			first, last, err := p.pageRangeForFile(pdf, c.id)
			if ok := c.receiveError(err, -1); !ok {
				continue
			}

			pipe, err = c.spwanCmdForPipe(pdf, first, last)
			if ok := c.receiveError(err, -1); !ok {
				continue
			}

			// initialize new file conversion progress
			c.setInit(pdf, first, last)
			c.t.PushTotal(c.total)
			ch = make(chan []string, last-first+1)
			go c.parseProgress(pipe, ch, first)
		}

		select {
		case <-p.ctx.Done():
			c.receiveError(errors.WithStack(p.ctx.Err()), c.current)
			return
		case entry, more := <-ch:
			if !more {
				c.cmd = nil
				c.setWaiting()
			} else {
				c.receiveEntry(entry)
			}
		}
	}
}

func (c *convertor) onComplete() {
	close(c.done)
	c.t.wg.Done()
}

func (c *convertor) completed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}
