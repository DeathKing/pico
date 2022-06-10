package gopdf2image

import (
	"context"
	"fmt"
	"log"
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

var dummyFn = func() {}

type CallOption func(o *Parameters, command []string) []string

type Parameters struct {
	pdfPath     string
	popplerPath string
	userPw      string
	ownerPw     string
	timeout     time.Duration

	// These fields are used by Convert Function
	dpi             int
	size            int
	firstPage       int32
	lastPage        int32
	workerCount     int32
	perPageTimeout  time.Duration
	fmt             string
	jpegOpt         map[string]string
	outputFile      string
	outputFolder    string
	outputFileFn    func(string, int32, int32, int32) string
	outputFolderFn  func(string, int32, int32, int32) string
	progress        bool
	singleFile      bool
	verbose         bool
	strict          bool
	transparent     bool
	grayscale       bool
	useCropBox      bool
	usePdftocario   bool
	hideAnnotations bool

	// these are what must be computed
	binary           string
	pageCount        int32
	minPagePerWorker int32

	ctx     context.Context
	cancel  context.CancelFunc
	clearFn func()

	// this field is only used by GetPDFInfo() call
	rawDates bool
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
		// remember current cancelFunc, which we may chian it later
		kancel := p.cancel
		ctx, cancle := context.WithTimeout(p.ctx, timeout)
		p.ctx, p.cancel = ctx, func() { cancle(); kancel() }

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

// WithSize sets the size of the resulting image(s), uses the Pillow (width, height) standard
// FIXME: not deal with size for now
func WithSize(size int) CallOption {
	return func(p *Parameters, command []string) []string {
		panic("not implemented yet")
		p.size = size
		return command
	}
}

// WithFirstPage sets the first page to convert
func WithFirstPage(firstPage int) CallOption {
	return func(p *Parameters, command []string) []string {
		p.firstPage = int32(firstPage)
		return command
	}
}

// WithLastPage sets the last page to convert
func WithLastPage(lastPage int) CallOption {
	return func(p *Parameters, command []string) []string {
		p.lastPage = int32(lastPage)
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

// WithWorkerCount sets the number of threads to use
func WithWorkerCount(workerCount int) CallOption {
	if workerCount < 1 {
		workerCount = 1
	}
	return func(p *Parameters, command []string) []string {
		p.workerCount = int32(workerCount)
		return command
	}
}

func WithPerPageTimeout(timeout time.Duration) CallOption {
	return func(p *Parameters, command []string) []string {
		p.perPageTimeout = timeout
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

// Write the resulting images to a folder (instead of directly in memory)
func WithOutputFolder(outputFolder string) CallOption {
	return func(p *Parameters, command []string) []string {
		p.outputFolder = outputFolder
		return command
	}
}

func WithProgress() CallOption {
	return func(p *Parameters, command []string) []string {
		p.progress = true
		return append(command, "-progress")
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
		p.cancel() // avoid context leaking
		p.ctx, p.cancel = ctx, dummyFn

		return command
	}
}

func defaultConvertCallOption() *Parameters {
	return &Parameters{
		dpi:            200,
		fmt:            "ppm",
		firstPage:      1,
		lastPage:       -1,
		workerCount:    1,
		timeout:        60 * time.Second,
		perPageTimeout: 10 * time.Second,

		ctx:    nil,
		cancel: func() {},

		binary: "pdftoppm",
	}
}

func defaultConvertFilesCallOption() *Parameters {
	p := defaultConvertCallOption()
	p.workerCount = 4

	return p
}

func (p *Parameters) applyAndBuildCommand(command []string, options ...CallOption) ([]string, error) {
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
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(
			newWrongArgumentError("hideAnnotations is not supported with pdftocairo"))
	}

	return command, nil
}

// Convert converts single PDF to images. This function is solely a options parser
// and command builder
func Convert(pdfPath string, options ...CallOption) (*Task, error) {
	call := defaultConvertCallOption()
	call.pdfPath = pdfPath

	command, err := call.applyAndBuildCommand([]string{}, options...)
	if err != nil {
		return nil, err
	}

	// 1. page calculation
	pages, err := GetPagesCount(pdfPath, options...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	totalPage := int32(pages)

	if call.lastPage < 0 || call.lastPage > totalPage {
		call.lastPage = totalPage
	}

	if call.firstPage > call.lastPage {
		err := newWrongArgumentError(
			fmt.Sprintf("invalid page range from %d to %d",
				call.firstPage, call.lastPage))
		return nil, errors.WithStack(err)
	}

	// 2. worker number calculation
	call.pageCount = call.lastPage - call.firstPage + 1

	// workerCount is not set, we could infer for one
	if call.workerCount <= 0 {
		switch {
		case call.pageCount > 50:
			call.workerCount = 3
		case call.pageCount > 20:
			call.workerCount = 2
		default:
			call.workerCount = 1
		}
	}

	if call.workerCount > call.pageCount {
		call.workerCount = call.pageCount
	}

	t := &Task{}

	// 3. convertor creation
	for i := int32(0); i < call.workerCount; i++ {
		t.buildConvertor(i, command)
	}

	// 4. start conversion

	// page calculation
	// path and folder calculation, could do later
	if call.outputFile == "" {
		base := filepath.Base(pdfPath)
		call.outputFile = base[:len(base)-len(filepath.Ext(base))]
	}

	if call.outputFolder == "" {
		call.outputFolder = filepath.Dir(pdfPath)
	}

	call.minPagePerWorker = call.pageCount / call.workerCount

	task := newConversion(call, call.pageCount)

	if call.progress {
		task.setInit(call.firstPage, call.lastPage, call.firstPage)
	}

	for i := int32(0); i < call.workerCount; i++ {
		task.SubTasks = append(
			task.SubTasks,
			task.createWorker(i, command),
		)
	}

	if err := task.Start(); err != nil {
		return nil, errors.WithStack(err)
	}

	return task, nil
}

// Convert converts single PDF to images. This function is solely a options parser
// and command builder
func ConvertBackup(pdfPath string, options ...CallOption) (*Task, error) {
	command := []string{}
	call := defaultConvertCallOption()
	call.pdfPath = pdfPath

	for _, option := range options {
		command = option(call, command)
	}

	// if no context is specified, we create a new one
	if call.ctx == nil {
		call.ctx, call.cancel = context.WithTimeout(context.Background(), call.timeout)
		call.clearFn = func() { call.cancel() }
	}

	if call.outputFile == "" {
		base := filepath.Base(pdfPath)
		call.outputFile = base[:len(base)-len(filepath.Ext(base))]
	}

	if call.outputFolder == "" {
		call.outputFolder = filepath.Dir(pdfPath)
	}

	if call.usePdftocario && call.fmt == "ppm" {
		call.fmt = "png"
	}

	pages, err := GetPagesCount(pdfPath, options...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	totalPage := int32(pages)

	// We start by getting the output format, the buffer processing function and if we need pdftocairo
	// parsedFormat, finalExtension, parseBufferFunc, usePdfcairoFormat := parseFormat(call.fmt, call.grayscale)
	parsedFormat, _, _, usePdfcairoFormat := parseFormat(call.fmt, call.grayscale)

	switch parsedFormat {
	case "jpeg":
		command = append(command, "-jpeg")
	case "png":
		command = append(command, "-png")
	case "tiff":
		command = append(command, "-tiff")
	}

	usePdfCairo := call.usePdftocario || usePdfcairoFormat ||
		(call.transparent && transparentFileType[parsedFormat])

	if usePdfCairo {
		call.binary = "pdftocairo"
	}

	version, err := getPopplerVersion(call.ctx, call.binary, call.popplerPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	major, minor := version[0], version[1]

	if major == 0 {
		if minor <= 57 {
			call.jpegOpt = nil
		}
		if minor <= 83 {
			call.hideAnnotations = false
		}
	}

	if usePdfCairo && call.hideAnnotations {
		return nil, errors.WithStack(
			newWrongArgumentError("hideAnnotations is not supported with pdftocairo"))
	}

	if call.lastPage < 0 || call.lastPage > totalPage {
		call.lastPage = totalPage
	}

	if call.firstPage > call.lastPage {
		err := newWrongArgumentError(
			fmt.Sprintf("invalid page range from %d to %d",
				call.firstPage, call.lastPage))
		return nil, errors.WithStack(err)
	}

	call.pageCount = call.lastPage - call.firstPage + 1
	if call.workerCount > call.pageCount {
		call.workerCount = call.pageCount
	}

	call.minPagePerWorker = call.pageCount / call.workerCount

	task := newConversion(call, call.pageCount)

	if call.progress {
		task.setInit(call.firstPage, call.lastPage, call.firstPage)
	}

	for i := int32(0); i < call.workerCount; i++ {
		task.SubTasks = append(
			task.SubTasks,
			task.createWorker(i, command),
		)
	}

	if err := task.Start(); err != nil {
		return nil, errors.WithStack(err)
	}

	return task, nil
}

// ConvertFiles converts multiple PDF files to images
//
// files could be type `[]string`, `chan string`
func ConvertFiles(files interface{}, options ...CallOption) (*Task, error) {
	return nil, nil
}
