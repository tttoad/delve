//go:build windows
// +build windows

package proc

import (
	"io"
	"os"
)

func NamedPipe() (reader io.ReadCloser, output OutputRedirect, err error) {
	reader, output.File, err = os.Pipe()

	return reader, output, err
}
