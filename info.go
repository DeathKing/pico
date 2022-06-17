package gopdf2image

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func defaultGetInfoCallArguments() *Parameters {
	ctx, cancel := context.WithCancel(context.Background())
	return &Parameters{
		ctx:     ctx,
		cancel:  cancel,
		timeout: 10 * time.Second,
	}
}

func GetInfo(pdf string, options ...CallOption) (map[string]string, error) {
	p := defaultGetInfoCallArguments()

	for _, option := range options {
		option(p, nil)
	}

	if _, err := os.Stat(pdf); errors.Is(err, os.ErrNotExist) {
		return nil, errors.WithStack(err)
	}

	command := []string{
		getCommandPath("pdfinfo", p.popplerPath),
		pdf,
	}

	if p.userPw != "" {
		command = append(command, "-upw", p.userPw)
	}

	if p.ownerPw != "" {
		command = append(command, "-opw", p.ownerPw)
	}

	if p.rawDates {
		command = append(command, "-rawdates")
	}

	if p.timeout > 0 {
		p.ctx, p.cancel = context.WithTimeout(p.ctx, p.timeout)
		defer p.cancel()
	}

	cmd := buildCmd(p.ctx, p.popplerPath, command)
	if p.verbose {
		fmt.Println("Call using ", cmd.String())
	}

	buf, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	infos := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(buf))

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "I/O Error:") {
			if p.verbose {
				fmt.Println("Error:", scanner.Text())
			}
			continue
		}
		pairs := strings.Split(scanner.Text(), ":")
		if len(pairs) == 2 {
			infos[pairs[0]] = strings.TrimSpace(pairs[1])
		}
	}
	return infos, nil
}

func GetPagesCount(pdfPath string, options ...CallOption) (int, error) {
	infos, err := GetInfo(pdfPath, options...)
	if err != nil {
		return 0, err
	}

	pages, ok := infos["Pages"]
	if !ok {
		return 0, errors.New("missing 'Pages' entry")
	}

	return strconv.Atoi(pages)
}
