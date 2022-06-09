package gopdf2image

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func defaultGetInfoCallArguments() *Parameters {
	return &Parameters{timeout: 10 * time.Second}
}

func GetInfo(pdfPath string, options ...CallOption) (map[string]string, error) {
	call := defaultGetInfoCallArguments()

	for _, option := range options {
		option(call, nil)
	}

	if _, err := os.Stat(pdfPath); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	command := []string{
		getCommandPath("pdfinfo", call.popplerPath),
		pdfPath,
	}

	if call.userPw != "" {
		command = append(command, "-upw", call.userPw)
	}

	if call.ownerPw != "" {
		command = append(command, "-opw", call.ownerPw)
	}

	if call.rawDates {
		command = append(command, "-rawdates")
	}

	var ctx context.Context
	var cancle context.CancelFunc
	if ctx = call.ctx; ctx == nil {
		ctx, cancle = context.WithTimeout(context.Background(), call.timeout)
		defer cancle()
	}

	cmd := buildCmd(ctx, call.popplerPath, command)
	buf, err := cmd.CombinedOutput()

	// fmt.Println(cmd.String())

	if err != nil {
		return nil, errors.WithStack(err)
	}

	infos := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(buf))

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "I/O Error:") {
			continue
		}
		pairs := strings.Split(scanner.Text(), ":")
		if len(pairs) == 2 {
			infos[pairs[0]] = strings.TrimSpace(pairs[1])
		}
	}
	return infos, nil
}
