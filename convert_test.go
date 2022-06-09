package gopdf2image

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
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
		t.Fatalf("%s: expected %d files, got %d", kase, expect, got)
	}
}

type subtest struct {
	title   string
	options []CallOption
	check   func(t *testing.T, task *Conversion)
}

func TestPDFConversionsInMultipleOptionCombiantion(t *testing.T) {
	subtests := []subtest{
		{
			title:   "TestSingleWorkerConversion",
			options: []CallOption{},
			check:   func(t *testing.T, task *Conversion) {},
		},
		{
			title: "TestMultipleWorkerConversion",
			options: []CallOption{
				WithWorkerCount(4),
			},
			check: func(t *testing.T, task *Conversion) {},
		},
		{
			title: "TestMultipleWorkerConversionWithProgress",
			options: []CallOption{
				WithWorkerCount(4),
				WithProgress(),
			},
			check: func(t *testing.T, task *Conversion) {},
		},
	}

	for _, sub := range subtests {
		t.Run(sub.title, func(t *testing.T) {
			for pdf, pageCount := range pdfs1 {
				dir := t.TempDir()
				task, err := Convert(fmt.Sprintf("%s%s", folder, pdf),
					append(sub.options, WithOutputFolder(dir))...)
				if err != nil {
					t.Fatalf("%+v", err)
				}

				task.Wait()
				if len(task.Errors()) > 0 {
					t.Fatalf("%+v", task.Errors())
				}

				mustContainsNFilesInDir(t, pdf, dir, pageCount)

				// further customize check
				sub.check(t, task)
			}
		})
	}
}

func TestPDFConversionsWithSingleWorker(t *testing.T) {
	for pdf, pageCount := range pdfs1 {
		dir := t.TempDir()
		task, err := Convert(fmt.Sprintf("%s%s", folder, pdf),
			WithOutputFolder(dir),
		)
		if err != nil {
			t.Fatalf("%+v", err)
		}

		task.Wait()
		mustContainsNFilesInDir(t, pdf, dir, pageCount)
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

// func TestGetPopplerVersion(t *testing.T) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	versions, err := getPopplerVersion(ctx, "pdftocairo", "")
// 	if err != nil {
// 		t.Error(err)
// 	}

// 	t.Log(versions)
// }

func TestInvalidPDFRange(t *testing.T) {
	_, err := Convert(fmt.Sprintf("%s%s", folder, "test.pdf"),
		WithOutputFolder(t.TempDir()),
		WithPageRange(42, 24),
	)
	assert.Contains(t, err.Error(), "invalid page range")
}

func TestCorruptedFileConversion(t *testing.T) {
	_, err := Convert(fmt.Sprintf("%s%s", folder, "test_corrupted.pdf"),
		WithOutputFolder(t.TempDir()),
	)
	var exitError *exec.ExitError
	assert.ErrorAs(t, err, &exitError)
}

// unfortunately, this is very hard to test
func TestPerPageTimeout(t *testing.T) {
	task, err := Convert(fmt.Sprintf("%s%s", folder, "test_241.pdf"),
		WithOutputFolder(t.TempDir()),
		WithProgress(),
		WithPerPageTimeout(time.Millisecond*10),
	)

	assert.NoError(t, err, "conversion task initialization should not failed")
	task.Wait()

	var errPerPageTimeout *PerPageTimeoutError
	assert.ErrorAs(t, task.Error(), &errPerPageTimeout)
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

	assert.ErrorIs(t, task.Error(), context.DeadlineExceeded)
}

func TestConversionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task, err := Convert(fmt.Sprintf("%s%s", folder, "test_241.pdf"),
		WithOutputFolder(t.TempDir()),
		WithContext(ctx),
	)

	go func() {
		time.Sleep(time.Second * 2)
		cancel()
	}()

	assert.NoError(t, err, "conversion task initialization should not failed")
	task.Wait()

	assert.ErrorIs(t, task.Error(), context.Canceled)
}

type strictModeTestCase struct {
	title   string
	options []CallOption
	check   func(t *testing.T, task *Conversion)
}

func TestStrictMode(t *testing.T) {
	subtests := []strictModeTestCase{
		{
			title:   "TestStrictMode",
			options: []CallOption{WithStrict()},
			check: func(t *testing.T, task *Conversion) {
				var errPDFSyntaxError *PDFSyntaxError
				assert.ErrorAs(t, task.Error(), &errPDFSyntaxError)
			},
		},
		{
			title: "TestStrictModeOff",
			check: func(t *testing.T, task *Conversion) {
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
