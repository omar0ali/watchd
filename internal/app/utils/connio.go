package utils

import (
	"bufio"
	"io"
	"net"
	"strings"
)

const EndMarker = "__END__"

// TODO:
type DaemonProcessFunc func(line string) string

type ConnIO struct {
	r *bufio.Reader
	w *bufio.Writer
}

func NewConnIO(conn net.Conn) *ConnIO {
	return &ConnIO{
		r: bufio.NewReader(conn),
		w: bufio.NewWriter(conn),
	}
}

func (c *ConnIO) ReadLine() (string, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func (c *ConnIO) WriteLine(msg string) error {
	_, err := c.w.WriteString(msg + "\n")
	if err != nil {
		return err
	}
	return c.w.Flush()
}

func (c *ConnIO) WriteResponse(msg string) error {
	lines := strings.SplitSeq(msg, "\n")

	for line := range lines {
		if err := c.WriteLine(line); err != nil {
			return err
		}
	}

	return c.WriteLine(EndMarker)
}

func (c *ConnIO) ReadResponse() ([]string, error) {
	var lines []string

	for {
		line, err := c.ReadLine()
		if err != nil {
			return nil, err
		}

		if line == EndMarker {
			break
		}

		lines = append(lines, line)
	}

	return lines, nil
}
