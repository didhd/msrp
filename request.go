package msrp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Request struct {
	toPath    string
	fromPath  string
	body      string
	message   string
	tid       string
	sender    string
	recipient string
	isCpim    bool
}

func (r *Request) write(w io.Writer) (err error) {
	// Wrap the writer in a bufio Writer if it's not already buffered.
	// Don't always call NewWriter, as that forces a bytes.Buffer
	// and other small bufio Writers to have a minimum 4k buffer
	// size.
	var bw *bufio.Writer
	if _, ok := w.(io.ByteWriter); !ok {
		bw = bufio.NewWriter(w)
		w = bw
	}

	fmt.Fprintf(w, r.body)

	return nil
}

func NewRequest(toPath, fromPath, sender, recipient, message string) (*Request, error) {
	req := &Request{
		tid:       randomString(12),
		toPath:    toPath,
		fromPath:  fromPath,
		sender:    sender,
		recipient: recipient,
		message:   message,
	}

	if message == "" {
		req.body = "MSRP {tid} SEND\r\n" +
			"To-Path: {toPath}\r\n" +
			"From-Path: {fromPath}\r\n" +
			"-------{tid}$\r\n"
		req.body = strings.Replace(req.body, "{tid}", req.tid, -1)
		req.body = strings.Replace(req.body, "{toPath}", toPath, -1)
		req.body = strings.Replace(req.body, "{fromPath}", fromPath, -1)
		return req, nil
	}

	payload := "From: <sip:anonymous@anonymous.invalid>\r\n" +
		"To: <sip:anonymous@anonymous.invalid>\r\n" +
		"DateTime: {datetime}\r\n" +
		"NS: imdn <urn:ietf:params:imdn>\r\n" +
		"imdn.Message-ID: {imdnMessageID}\r\n" +
		"imdn.Disposition-Notification: positive-delivery, display\r\n\r\n" +

		"Content-type: text/plain;charset=UTF-8\r\n" +
		"Content-Length: {contentLength}\r\n\r\n" +

		"{body}\r\n"

	// Format payload first
	payload = strings.Replace(payload, "{datetime}", getDatetime(), -1)
	imdnMessageID := randomString(8) + "-" + randomString(4) + "-" + randomString(4) + "-" + randomString(4) + "-" + randomString(10)
	payload = strings.Replace(payload, "{imdnMessageID}", imdnMessageID, -1)
	contentLength := strconv.Itoa(len(message))
	payload = strings.Replace(payload, "{contentLength}", contentLength, -1)
	payload = strings.Replace(payload, "{body}", message, -1)

	req.body = "MSRP {tid} SEND\r\n" +
		"To-Path: {toPath}\r\n" +
		"From-Path: {fromPath}\r\n" +
		"Message-ID: {mid}\r\n" +
		"Success-Report: no\r\n" +
		"Failure-Report: yes\r\n" +
		"Byte-Range: {byterange}\r\n" +
		"Content-Type: message/cpim\r\n\r\n" +

		payload +

		"-------{tid}$\r\n"

	// Format body
	req.body = strings.Replace(req.body, "{tid}", req.tid, -1)
	req.body = strings.Replace(req.body, "{toPath}", toPath, -1)
	req.body = strings.Replace(req.body, "{fromPath}", fromPath, -1)
	req.body = strings.Replace(req.body, "{mid}", randomString(6), -1)
	byterange := "1-" + strconv.Itoa(len(payload)) + "/" + strconv.Itoa(len(payload))
	req.body = strings.Replace(req.body, "{byterange}", byterange, -1)

	return req, nil
}
