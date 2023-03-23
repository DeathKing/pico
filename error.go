package pico

import (
	"fmt"

	"github.com/pkg/errors"
)

type GetBinaryVersionError struct {
	msg string
}

type PerPageTimeoutError struct {
	msg string
}

type PDFSyntaxError struct {
	msg string
}

type WrongArgumentError struct {
	msg string
}

type ConversionError struct {
	pdf      string
	page     int32
	workerId int32
	err      error
}

func (e *ConversionError) Cause() error {
	return e.err
}

func (e *ConversionError) Error() string {
	worker, page := "", ""
	if e.workerId >= 0 {
		worker = fmt.Sprintf(" by worker#%02d", e.workerId)
	}

	if e.page >= 0 {
		page = fmt.Sprintf(" at page %d", e.page)
	}

	return fmt.Sprintf("failed to convert %s%s%s: %s", e.pdf, worker, page, e.err)
}

var ErrProviderClosed = errors.New("provider is closed")

func NewPerPageTimeoutError(page string) *PerPageTimeoutError {
	return &PerPageTimeoutError{
		msg: fmt.Sprintf("processing page %s timeout", page),
	}
}

func (e *PerPageTimeoutError) Error() string {
	return e.msg
}

func newWrongArgumentError(detail string) *WrongArgumentError {
	return &WrongArgumentError{
		msg: fmt.Sprintf("wrong argument: %s", detail),
	}
}

// Wrong page range given: the first page (21) can not be after the last page (14).

func newWrongPageRangeError(first, last int32) *WrongArgumentError {
	return newWrongArgumentError(fmt.Sprintf("the first page (%d) can not be after the last page (%d)", first, last))
}

func (e *WrongArgumentError) Error() string {
	return e.msg
}

func NewGetBinaryVersionError(binary string) *GetBinaryVersionError {
	return &GetBinaryVersionError{
		msg: fmt.Sprintf("failed to get version of %s binary", binary),
	}
}

func (e *GetBinaryVersionError) Error() string {
	return e.msg
}

func NewOldPDFSyntaxError(line, filename string, page int32) *PDFSyntaxError {
	return &PDFSyntaxError{
		msg: fmt.Sprintf("syntax error was thrown during rendering %s at page %d: %s",
			filename, page, line),
	}
}

func NewPDFSyntaxError(line string) *PDFSyntaxError {
	return &PDFSyntaxError{
		msg: fmt.Sprintf("poppler: %s", line),
	}
}

func (e *PDFSyntaxError) Error() string {
	return e.msg
}
