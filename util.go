package gopdf2image

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var _versionRE = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

func getPopplerVersion(ctx context.Context, binary, popplerPath string) ([]int, error) {
	command := []string{}
	command = append(command, getCommandPath(binary, popplerPath), "-v")

	cmd := buildCmd(ctx, popplerPath, command)
	buf, err := cmd.CombinedOutput()

	if err != nil {
		return nil, errors.Wrapf(err, "getPopplerVersion: ")
	}

	matches := _versionRE.FindStringSubmatch(string(buf))
	if len(matches) < 4 {
		return nil, errors.WithStack(NewGetBinaryVersionError(binary))
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])

	return []int{major, minor}, nil
}

func getCommandPath(binary, popplerPath string) string {
	// it seems redundant to add `.exe` extension to the binary name,
	// but the Python version pdf2image does so.
	if runtime.GOOS == "windows" {
		binary = binary + ".exe"
	}

	if popplerPath != "" {
		binary = filepath.Join(popplerPath, binary)
	}

	return binary
}

func buildCmd(ctx context.Context, popplerPath string, command []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = append([]string{}, os.Environ()...)
	if popplerPath != "" {
		cmd.Env = append(cmd.Env, "LD_LIBRARY_PATH="+popplerPath+":"+os.Getenv("LD_LIBRARY_PATH"))
	}

	return cmd
}

func parseFormat(format string, grayscale bool) (string, string, bool) {
	format = strings.ToLower(format)

	if format[0] == '.' {
		format = format[1:]
	}

	switch {
	case format == "jpeg" || format == "jpg":
		return "jpeg", "jpg", false

	case format == "png":
		return "png", "png", false

	case format == "tiff" || format == "tif":
		return "tiff", "tif", true

	case format == "ppm" && grayscale:
		return "ppm", "pgm", false

	default:
		return "ppm", "ppm", false
	}
}
