package httpflv

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type HttpReader struct {
}

func (hr *HttpReader) ReadMessage(r io.Reader) error {
	reader := bufio.NewReader(r)

	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		fmt.Println("Debug.... ", line, len(line))
		if line == "\r\n" {
			break
		} else if strings.HasPrefix(line, "Content-Length") {
			sperates := strings.SplitN(line, ":", 1)
			if len(sperates) != 2 {
				return fmt.Errorf("bad content length:%s", line)
			}

			strLength := sperates[1]
			strLength = strings.TrimRight(strLength, "\r\n")
			strLength = strings.TrimSpace(strLength)

			if contentLength, err = strconv.Atoi(strLength); err != nil {
				return fmt.Errorf("bad content length:%s", line)
			}
		}
	}

	if contentLength > 0 {
		rbuf := make([]byte, contentLength)
		n, err := reader.Read(rbuf)
		if err != nil {
			if err != io.EOF {
				return err
			}
		}

		if n != contentLength {
			return fmt.Errorf("read not less than %d", n)
		}
	}
	return nil
}
