package gopdf2image

import (
	"fmt"
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

func NewPerPageTimeoutError(page int32) *PerPageTimeoutError {
	return &PerPageTimeoutError{
		msg: fmt.Sprintf("processing page %d timeout", page),
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
		msg: fmt.Sprintf("got error from poppler: %s", line),
	}
}

func (e *PDFSyntaxError) Error() string {
	return e.msg
}
