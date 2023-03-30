package pico

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var jpegOptMap = map[string]interface{}{
	"quality":     nil,
	"optimize":    nil,
	"progressive": nil,
}

var transparentFileType = map[string]bool{
	"png":  true,
	"tiff": true,
}

type nameFn func(pdf string, index int32, first, last int32) string

type CallOption func(o *Parameters, command []string) []string

type Parameters struct {
	popplerPath string
	userPw      string
	ownerPw     string
	options     []CallOption
	timeout     time.Duration

	// These fields are used by Convert Function
	dpi             int
	firstPage       int32
	lastPage        int32
	job             int32
	fmt             string
	jpegOpt         map[string]string
	outputFile      string
	outputFolder    string
	outputFileFn    nameFn
	outputFolderFn  nameFn
	singleFile      bool
	verbose         bool
	strict          bool
	transparent     bool
	grayscale       bool
	useCropBox      bool
	usePdftocario   bool
	hideAnnotations bool

	scaleTo  int
	scaleToX int
	scaleToY int

	// these are what must be computed
	baseCommand       []string
	binary            string
	pageCount         int32
	minPagesPerWorker int32

	ctx    context.Context
	cancel context.CancelFunc

	// this field is only used by GetPDFInfo() call
	rawDates bool
}

// pageRangeForPart calculates the page range needed to be converted for a given file
// by specific worker during Convert() call.
func (p *Parameters) pageRangeForPart(pdf string, index int32) (int32, int32, error) {
	reminder := p.pageCount % p.job

	amortization := int32(0)
	if index < reminder {
		amortization = 1
	}

	first := p.firstPage + index*p.minPagesPerWorker
	last := first + reminder + p.minPagesPerWorker - 1 + amortization

	// FIXME: seems redundant
	if last > p.lastPage {
		panic("Wrong calculation, worker lastPage should not greater than task lastPage")
	}

	return first, last, nil
}

// pagePrangeForFile calculates the page range needed to be converted for a given file
// by specific worker during ConvertFiles() call.
func (p *Parameters) pageRangeForFile(pdf string, index int32) (int32, int32, error) {
	first, last := p.firstPage, p.lastPage

	pages, err := GetPagesCount(pdf, p.options...)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get pages count ")
	}

	totalPage := int32(pages)

	if last < 0 || last > totalPage {
		last = totalPage
	}

	if first > last {
		return 0, 0, errors.WithStack(newWrongPageRangeError(first, last))
	}

	return first, last, nil
}

// buildCommand builds the full command line to convert (part of) a PDF file,
// outputFile and outputFolder are calculated during this call based on the
// index of the worker/convertor.
func (p *Parameters) buildCommand(pdf string, index, first, last int32) []string {
	outputFile := p.outputFile
	if outputFile == "" {
		ext := path.Ext(pdf)
		outputFile = pdf[:len(pdf)-len(ext)]
	}

	if p.outputFileFn != nil {
		outputFile = p.outputFileFn(pdf, index, first, last)
	}

	outputFolder := p.outputFolder
	if p.outputFolderFn != nil {
		outputFolder = p.outputFolderFn(pdf, index, first, last)
	}

	outputFile = filepath.Join(outputFolder, outputFile)
	os.MkdirAll(filepath.Dir(outputFile), 0755)

	command := []string{
		getCommandPath(p.binary, p.popplerPath),
		"-progress",
		"-f", strconv.Itoa(int(first)),
		"-l", strconv.Itoa(int(last)),
	}
	command = append(command, p.baseCommand...)
	command = append(command, pdf, outputFile)

	return command
}

func (p *Parameters) apply(options ...CallOption) error {
	command := []string{}
	for _, option := range options {
		command = option(p, command)
	}

	if p.usePdftocario && p.fmt == "ppm" {
		p.fmt = "png"
	}

	parsedFormat, _, usePdfcairoFormat := parseFormat(p.fmt, p.grayscale)

	switch parsedFormat {
	case "jpeg":
		command = append(command, "-jpeg")
	case "png":
		command = append(command, "-png")
	case "tiff":
		command = append(command, "-tiff")
	}

	usePdfCairo := p.usePdftocario || usePdfcairoFormat ||
		(p.transparent && transparentFileType[parsedFormat])

	if usePdfCairo {
		p.binary = "pdftocairo"
	}

	// this considered as a Fatal if we cannot get the version of poppler utilities
	version, err := getPopplerVersion(p.ctx, p.binary, p.popplerPath)
	if err != nil {
		return errors.WithStack(err)
	}

	major, minor := version[0], version[1]

	if major == 0 {
		if minor <= 57 {
			p.jpegOpt = nil
		}
		if minor <= 83 {
			p.hideAnnotations = false
		}
	}

	if usePdfCairo && p.hideAnnotations {
		return errors.WithStack(
			newWrongArgumentError("hideAnnotations is not supported with pdftocairo"))
	}

	// size related options
	if p.scaleTo > 0 {
		command = append(command, "-scale-to", strconv.Itoa(p.scaleTo))
	} else {
		if p.scaleToX > 0 {
			command = append(command, "-scale-to-x", strconv.Itoa(p.scaleToX))
		}
		if p.scaleToY > 0 {
			command = append(command, "-scale-to-y", strconv.Itoa(p.scaleToY))
		}
	}

	// ctx
	if p.timeout > 0 {
		p.ctx, p.cancel = context.WithTimeout(p.ctx, p.timeout)
	}
	p.options = options
	p.baseCommand = command

	return nil
}

// WithPopplerPath sets poppler binaries lookup path
func WithPopplerPath(popplerPath string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.popplerPath = popplerPath
		return command
	}
}

// WithUserPw sets PDF's password
func WithUserPw(userPw string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.userPw = userPw
		return append(command, "-upw", userPw)
	}
}

// WithOwnerPw sets PDF's owner password
func WithOwnerPw(ownerPw string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.ownerPw = ownerPw
		return append(command, "-opw", ownerPw)
	}
}

// WithTimeout
func WithTimeout(timeout time.Duration) CallOption {
	return func(p *Parameters, command []string) []string {
		p.timeout = timeout

		return command
	}
}

// WithDpi sets image quality in DPI (default 200)
func WithDpi(dpi int) CallOption {
	// this is the ClientOption function type
	return func(p *Parameters, command []string) []string {
		p.dpi = dpi
		return command
	}
}

// WithScaleTo sets the size of the resulting images, size=400 will fit the
// image to a 400x400 box, preserving aspect ratio
func WithScaleTo(size int) CallOption {
	return func(p *Parameters, command []string) []string {
		p.scaleTo = size
		return command
	}
}

// WithSize is the alias of WithScaleTo
var WithSize = WithScaleTo

func WithScaleToX(size int) CallOption {
	return func(p *Parameters, command []string) []string {
		p.scaleToX = size
		return command
	}
}

func WithScaleToY(size int) CallOption {
	return func(p *Parameters, command []string) []string {
		p.scaleToY = size
		return command
	}
}

// WithFirstPage sets the first page to convert
func WithFirstPage(firstPage int) CallOption {
	return func(p *Parameters, command []string) []string {
		if firstPage >= 0 {
			p.firstPage = int32(firstPage)
		}
		return command
	}
}

// WithLastPage sets the last page to convert
func WithLastPage(lastPage int) CallOption {
	return func(p *Parameters, command []string) []string {
		if lastPage >= 0 {
			p.lastPage = int32(lastPage)
		}
		return command
	}
}

// WithPageRange sets the range of pages to convert
func WithPageRange(firstPage, lastPage int) CallOption {
	return func(p *Parameters, command []string) []string {
		p.firstPage = int32(firstPage)
		p.lastPage = int32(lastPage)
		return command
	}
}

// WithJob sets the number of threads to use
func WithJob(job int) CallOption {
	if job < 1 {
		job = 1
	}
	return func(p *Parameters, command []string) []string {
		p.job = int32(job)
		return command
	}
}

// WithFormat sets the output image format
func WithFormat(fmt string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.fmt = fmt
		return command
	}
}

func WithJPEGQuality(quality int) CallOption {
	return func(p *Parameters, command []string) []string {
		if quality < 0 || quality > 100 {
			quality = 75
		}
		p.jpegOpt["quality"] = strconv.Itoa(quality)
		return command
	}
}

func WithJPEGOptimize(optimize bool) CallOption {
	return func(p *Parameters, command []string) []string {
		if optimize {
			p.jpegOpt["optimize"] = "y"
		} else {
			p.jpegOpt["optimize"] = "n"
		}
		return command
	}
}

func WithJPEGProgressive(progressive bool) CallOption {
	return func(p *Parameters, command []string) []string {
		if progressive {
			p.jpegOpt["progressive"] = "y"
		} else {
			p.jpegOpt["progressive"] = "n"
		}
		return command
	}
}

func WithJPEGOpt(jpegOpt map[string]string) CallOption {
	for k := range jpegOpt {
		if _, ok := jpegOptMap[k]; !ok {
			log.Fatal("Invalid JPEG option: " + k)
		}
	}

	return func(p *Parameters, command []string) []string {
		p.jpegOpt = jpegOpt

		parts := []string{}
		for k, v := range jpegOpt {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}

		return append(command, "-jpegopt", strings.Join(parts, ","))
	}
}

func WithOutputFile(outputFile string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.outputFile = outputFile
		return command
	}
}

func WithOutputFileFn(fn nameFn) CallOption {
	return func(p *Parameters, command []string) []string {
		p.outputFileFn = fn
		return command
	}
}

// Write the resulting images to a folder (instead of directly in memory)
func WithOutputFolder(outputFolder string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.outputFolder = outputFolder
		return command
	}
}

func WithSingleFile() CallOption {
	return func(p *Parameters, command []string) []string {
		p.singleFile = true
		return append(command, "-singlefile")
	}
}

// WithVerbose will prints useful debugging information
func WithVerbose() CallOption {
	return func(p *Parameters, command []string) []string {
		p.verbose = true
		return command
	}
}

// WithStrict sets to strict mode, when a Syntax Error is thrown, it will be raised as an Exception
func WithStrict() CallOption {
	return func(p *Parameters, command []string) []string {
		p.strict = true
		return command
	}
}

func WithTransparent() CallOption {
	return func(p *Parameters, command []string) []string {
		p.transparent = true
		return command
	}
}

func WithGrayScale() CallOption {
	return func(p *Parameters, command []string) []string {
		p.grayscale = true
		return append(command, "-grayscale")
	}
}

func WithUseCropBox() CallOption {
	return func(p *Parameters, command []string) []string {
		p.useCropBox = true
		return command
	}
}

func WithUsePdftocario() CallOption {
	return func(p *Parameters, command []string) []string {
		p.usePdftocario = true
		return command
	}
}

func WithHideAnnotations() CallOption {
	return func(p *Parameters, command []string) []string {
		p.hideAnnotations = true
		return command
	}
}

func WithContext(ctx context.Context) CallOption {
	return func(p *Parameters, command []string) []string {
		p.ctx = ctx

		return command
	}
}

func defaultConvertCallOption() *Parameters {
	ctx, cancel := context.WithCancel(context.Background())
	return &Parameters{
		dpi:       200,
		fmt:       "ppm",
		firstPage: 1,
		lastPage:  -1,
		job:       1,
		timeout:   -1,

		ctx:    ctx,
		cancel: cancel,

		binary: "pdftoppm",
	}
}

func defaultConvertFilesCallOption() *Parameters {
	p := defaultConvertCallOption()
	p.job = 4

	return p
}
