package msrp

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type Response struct {
	toPath   string
	fromPath string
	body     string
	tid      string
}

func (r *Response) write(w io.Writer) (err error) {
	var bw *bufio.Writer
	if _, ok := w.(io.ByteWriter); !ok {
		bw = bufio.NewWriter(w)
		w = bw
	}

	fmt.Fprintf(w, r.body)
	return nil
}

func NewResponse(toPath, fromPath, tid string) (*Response, error) {
	res := &Response{
		tid:      tid,
		toPath:   toPath,
		fromPath: fromPath,
	}

	res.body = "MSRP {tid} 200 OK\r\nTo-Path: {toPath}\r\nFrom-Path: {fromPath}\r\n-------{tid}$\r\n"

	// Format body
	res.body = strings.Replace(res.body, "{tid}", res.tid, -1)
	res.body = strings.Replace(res.body, "{toPath}", toPath, -1)
	res.body = strings.Replace(res.body, "{fromPath}", fromPath, -1)

	return res, nil
}
