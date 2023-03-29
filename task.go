package pico

import (
	"sync"

	"github.com/pkg/errors"
)

type Task struct {
	// progress is the total progress of a task
	progress

	// wg waits for all convertor completed
	wg *sync.WaitGroup

	// Convertors are used to convert PDF to images
	Convertors []*Convertor

	// params is the final computed arguments used to invoke the conversion call
	params *Parameters

	// Entries is the channel of conversion progress entry
	// the format will be ["currentPage" "lastPage" "filename"]
	Entries chan []string

	// done is the channel that, when it is closed, all the task is completed
	done chan interface{}
}

// SingleTask deals with single document conversion where usually the given pdf
// is a large file so we split it (evenly) into parts by page ranges and dispatch
// them to every convertor.
type SingleTask struct {
	Task
}

// BatchTask deals with multiple documents conversion where each convertor converts
// single document.
type BatchTask struct {
	Task
}

func (t *Task) wait() {
	t.wg.Wait()
	t.params.cancel()
	close(t.Entries)
	close(t.done)
}

func (t *Task) Completed() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// Wait hijacks the EntryChan and wait for all the workers finish
func (t *Task) Wait() {
	for range t.Entries {
	}
	<-t.done
}

// WaitAndCollect acts like Wait() but collects all the entries into a slice.
// A empty array is returned if there is no entry received.
func (t *Task) WaitAndCollect() (entries [][]string) {
	entries = make([][]string, 0)
	for entry := range t.Entries {
		entries = append(entries, entry)
	}
	<-t.done
	return entries
}

func (t *Task) Errors() (errs []*ConversionError) {
	<-t.done
	for _, c := range t.Convertors {
		errs = append(errs, c.Errors()...)
	}
	return
}

func (t *Task) Error() error {
	if errs := t.Errors(); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (t *Task) buildConvertor(index int32) *Convertor {
	return &Convertor{
		t:    t,
		id:   index,
		done: make(chan interface{}),
	}
}

func newSingleTask(p *Parameters) *SingleTask {
	return &SingleTask{Task{
		wg:     &sync.WaitGroup{},
		done:   make(chan interface{}),
		params: p,

		Entries: make(chan []string, p.pageCount),
	}}
}

func newBatchTask(p *Parameters) *BatchTask {
	return &BatchTask{Task{
		wg:     &sync.WaitGroup{},
		done:   make(chan interface{}),
		params: p,

		Entries: make(chan []string, 200),
	}}
}

// Start initiates the conversion process
func (t *SingleTask) Start(pdf string) error {
	for i := int32(0); i < t.params.job; i++ {
		c := t.buildConvertor(i)

		t.wg.Add(1)
		t.Convertors = append(t.Convertors, c)

		if err := c.start(pdf); err != nil {
			t.params.cancel()
			return errors.Wrap(err, "failed to start convertor")
		}
	}

	go t.wait()

	return nil
}

func (t *BatchTask) Start(provider PdfProvider) error {
	for i := int32(0); i < t.params.job; i++ {
		c := t.buildConvertor(i)

		t.wg.Add(1)
		t.Convertors = append(t.Convertors, c)
		go c.startAsWorker(provider)
	}

	go t.wait()

	return nil
}
