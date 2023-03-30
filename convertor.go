package pico

import (
	"bufio"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Convertor struct {
	Progress

	t   *Task
	id  int32
	cmd *exec.Cmd

	// converrs is a list of errors that occurred during the conversion.
	converrs []*ConversionError

	done chan interface{}

	aborted bool
}

// spwanCmdForPipe spwans an `exec.Cmd` for convererting the pdf from `first` to `last`,
//
func (c *Convertor) spwanCmdForPipe(pdf string, first, last int32) (io.ReadCloser, error) {
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

func (c *Convertor) Errors() []*ConversionError {
	return c.converrs
}

func (c *Convertor) Error() (err error) {
	if len(c.converrs) > 0 {
		err = c.converrs[0].err
	}
	return
}

// receiveError 可能存在跨pdf error 的情况么？
func (c *Convertor) receiveError(err error, page int32) bool {
	if err == nil {
		return true
	}

	c.converrs = append(c.converrs, &ConversionError{
		pdf:      c.pdf,
		page:     page,
		workerId: c.id,
		err:      err,
	})

	// if we're in `strict` mode, break further execution by return false
	return !c.t.params.strict
}

// receiveEntry is called when an entry is received from the parser. This
// function is usually used to
//   1. update the progress
//   2. send the entry to the task
func (c *Convertor) receiveEntry(entry []string) {
	current, _ := strconv.Atoi(entry[0])

	c.Incr(1)
	c.SetCurrent(int32(current))
	c.t.Entries <- append(entry, strconv.Itoa(int(c.id)))
}

// current total outputFileName
var _entryRE = regexp.MustCompile(`(\d+) (\d+) (.+)`)

func (c *Convertor) parseProgress(pipe io.ReadCloser, ch chan<- []string, current int32) {
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
func (c *Convertor) start(pdf string) error {
	p := c.t.params
	first, last, _ := p.pageRangeForPart(pdf, c.id)

	pipe, err := c.spwanCmdForPipe(pdf, first, last)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Progress.setInit(pdf, first, last)

	// ch is closed by `parseProgress`
	ch := make(chan []string, last-first+1)
	go c.parseProgress(pipe, ch, first)

	go func() {
		defer c.onComplete()
		for {
			select {
			case <-p.ctx.Done():
				c.receiveError(errors.WithStack(p.ctx.Err()), -1)
				c.aborted = true
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

func (c *Convertor) startAsWorker(provider PdfProvider) {
	var pdf string
	var pipe io.ReadCloser
	var more bool
	var ch chan []string

	defer c.onComplete()

	p := c.t.params

	// set the total number as long as we could get the file count from provider
	if cnt := provider.Count(); cnt > 0 {
		c.t.setInit("", 1, int32(cnt))
	}

	for {
		if c.cmd == nil {
			// accuquire a file for conversion
			select {
			case <-p.ctx.Done():
				c.receiveError(errors.WithStack(p.ctx.Err()), -1)
				c.aborted = true
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
			if provider.Count() == -1 {
				c.t.PushTotal(1)
			}

			ch = make(chan []string, last-first+1)
			go c.parseProgress(pipe, ch, first)
		}

		select {
		case <-p.ctx.Done():
			c.receiveError(errors.WithStack(p.ctx.Err()), c.current)
			c.aborted = true
			return
		case entry, more := <-ch:
			// no more entry means conversion has finised for that file
			if !more {
				c.cmd = nil
				c.setWaiting()
				c.t.Incr(1)
			} else {
				c.receiveEntry(entry)
			}
		}
	}
}

func (c *Convertor) onComplete() {
	close(c.done)
	c.t.wg.Done()
}

// Completed reports whether the convertor is in completed state
func (c *Convertor) Completed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Convertor) Aborted() bool {
	return false
}
