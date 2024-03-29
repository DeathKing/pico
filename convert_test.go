package pico

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const folder = "./tests/"

var pdfs1 = map[string]int{
	"test.pdf":     1,
	"test_14.pdf":  14,
	"test_241.pdf": 241,
}

func mustContainsNFilesInDir(t *testing.T, kase, dir string, expect int) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	if got := len(files); got != expect {
		t.Fatalf("%s: expected %d files in %s, got %d", kase, expect, dir, got)
	}
}

type subtest struct {
	title   string
	options []CallOption
	check   func(t *testing.T, task *Task)
}

func TestPDFConversionsInMultipleOptionCombiantion(t *testing.T) {
	commonOptions := []CallOption{WithStrict(), WithVerbose()}
	subtests := []subtest{
		{
			title: "TestSingleWorkerConversion",
			options: []CallOption{
				WithStrict(),
			},
			check: func(t *testing.T, task *Task) {},
		},
		{
			title: "TestMultipleWorkerConversion",
			options: []CallOption{
				WithJob(4),
			},
			check: func(t *testing.T, task *Task) {},
		},
		{
			title: "TestMultipleWorkerConversionWithProgress",
			options: []CallOption{
				WithJob(4),
			},
			check: func(t *testing.T, task *Task) {},
		},
	}

	for _, sub := range subtests {
		t.Run(sub.title, func(t *testing.T) {
			for pdf, pageCount := range pdfs1 {
				dir := t.TempDir()

				options := append(commonOptions, sub.options...)
				options = append(options, WithOutputFolder(dir))

				file := fmt.Sprintf("%s%s", folder, pdf)
				task, err := Convert(file, options...)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				if task.Wait(); len(task.Errors()) > 0 {
					t.Fatalf("%+v", task.Errors())
				}

				outputFolder := filepath.Dir(filepath.Join(dir, file))
				mustContainsNFilesInDir(t, pdf, outputFolder, pageCount)

				// further customize check
				sub.check(t, &task.Task)
			}
		})
	}
}

func TestGetPDFInfo(t *testing.T) {
	for pdf, pageCount := range pdfs1 {
		info, err := GetInfo(fmt.Sprintf("%s%s", folder, pdf))
		assert.NoErrorf(t, err, "GetInfo failed")

		assert.Containsf(t, info, "Pages", "missing 'Pages' entry for pdf %s", pdf)

		page, _ := strconv.Atoi(info["Pages"])
		if page != pageCount {
			t.Errorf("wrong page count for pdf %s, expect %d got %d", pdf, pageCount, page)
			t.Fail()
		}
	}
}

func TestCorruptedFileConversion(t *testing.T) {
	_, err := Convert(fmt.Sprintf("%s%s", folder, "test_corrupted.pdf"),
		WithOutputFolder(t.TempDir()),
	)
	var exitError *exec.ExitError
	assert.ErrorAs(t, err, &exitError)
}

func TestConversionTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	task, err := Convert(fmt.Sprintf("%s%s", folder, "test_241.pdf"),
		WithOutputFolder(t.TempDir()),
		WithContext(ctx),
	)

	assert.NoError(t, err, "conversion task initialization should not failed")
	task.Wait()

	assert.ErrorAs(t, task.Error(), &context.DeadlineExceeded)
}

func TestConversionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := Convert(fmt.Sprintf("%s%s", folder, "test_241.pdf"),
		WithOutputFolder(t.TempDir()),
		WithContext(ctx),
	)

	go func() {
		time.Sleep(time.Second * 1)
		cancel()
	}()

	assert.NoError(t, err, "conversion task initialization should not failed")
	task.Wait()

	assert.ErrorAs(t, task.Error(), &context.Canceled)
}

type strictModeTestCase struct {
	title   string
	options []CallOption
	check   func(t *testing.T, task *Task)
}

func TestStrictMode(t *testing.T) {
	subtests := []strictModeTestCase{
		{
			title:   "TestStrictMode",
			options: []CallOption{WithStrict()},
			check: func(t *testing.T, task *Task) {
				var errPDFSyntaxError *PDFSyntaxError
				assert.ErrorAs(t, task.Error(), &errPDFSyntaxError)
			},
		},
		{
			title: "TestStrictModeOff",
			check: func(t *testing.T, task *Task) {
				assert.NoError(t, task.Error())
			},
		},
	}

	for _, subtest := range subtests {
		t.Run(subtest.title, func(t *testing.T) {
			task, err := Convert(fmt.Sprintf("%s%s", folder, "test_strict.pdf"),
				append(subtest.options, WithOutputFolder(t.TempDir()))...)
			assert.NoError(t, err, "conversion task initialization should not failed")
			task.Wait()
		})
	}
}
