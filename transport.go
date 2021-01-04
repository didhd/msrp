package msrp

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// DefaultTransport is the default implementation of Transport and is
// used by DefaultClient. It establishes network connections as needed
// and caches them for reuse by subsequent calls.
var DefaultTransport = &Transport{}

type Transport struct {
	MsrpAddrs map[addrKey]*connectMethod
	MsrpConns map[connectMethodKey]*persistConn

	// WriteBufferSize specifies the size of the write buffer used
	// when writing to the transport.
	// If zero, a default (currently 4KB) is used.
	WriteBufferSize int

	// ReadBufferSize specifies the size of the read buffer used
	// when reading from the transport.
	// If zero, a default (currently 4KB) is used.
	ReadBufferSize int
}

func (t *Transport) writeBufferSize() int {
	if t.WriteBufferSize > 0 {
		return t.WriteBufferSize
	}
	return 4 << 10
}

func (t *Transport) readBufferSize() int {
	if t.ReadBufferSize > 0 {
		return t.ReadBufferSize
	}
	return 4 << 10
}

type addrKey struct {
	sender    string
	recipient string
}

type connectMethod struct {
	ToPath   string
	FromPath string
}

func (cm *connectMethod) key() connectMethodKey {
	return connectMethodKey{
		addr: strings.Split(cm.ToPath, "/")[2],
	}
}

type connectMethodKey struct {
	addr string
}

type persistConn struct {
	t        *Transport
	cm       connectMethod
	cacheKey connectMethodKey
	addrKey  addrKey
	conn     net.Conn
	writech  chan Message  // written by OneWay; read by writeLoop
	br       *bufio.Reader // from conn
	bw       *bufio.Writer // to conn
}

func (t *Transport) GetConnectionMethod(sender, recipient string) *connectMethod {
	ak := addrKey{
		sender:    sender,
		recipient: recipient,
	}
	if cm, ok := t.MsrpAddrs[ak]; ok {
		return cm
	}
	return nil
}

func (t *Transport) connectMethodForRequest(req *Request) connectMethod {
	return connectMethod{
		ToPath:   req.toPath,
		FromPath: req.fromPath,
	}
}

func (t *Transport) addrKeyForRequest(req *Request) addrKey {
	return addrKey{
		sender:    req.sender,
		recipient: req.recipient,
	}
}

func (t *Transport) registerMsrpAddr(req *Request) {
	ak := addrKey{
		sender:    req.sender,
		recipient: req.recipient,
	}
	t.MsrpAddrs[ak] = &connectMethod{
		ToPath:   req.toPath,
		FromPath: req.fromPath,
	}
}

func (t *Transport) OneWay(req *Request) (err error) {
	pconn, err := t.getConn(req)
	if err != nil {
		return err
	}

	mes := Message{}
	mes.req = req
	fmt.Println(req.body)
	pconn.writech <- mes
	return nil
}

func (t *Transport) getConn(req *Request) (pc *persistConn, err error) {
	if t.MsrpAddrs == nil {
		t.MsrpAddrs = make(map[addrKey]*connectMethod)
	}
	if t.MsrpConns == nil {
		t.MsrpConns = make(map[connectMethodKey]*persistConn)
	}

	cm := t.connectMethodForRequest(req)
	ck := cm.key()

	if pconn, ok := t.MsrpConns[ck]; ok { // Connection already exists.
		return pconn, nil
	} else {
		var err error
		conn, err := net.Dial("tcp", ck.addr)
		if err != nil {
			return nil, err
		}

		ak := t.addrKeyForRequest(req)

		pconn = &persistConn{
			t:        t,
			cm:       cm,
			cacheKey: ck,
			addrKey:  ak,
			conn:     conn,
			writech:  make(chan Message, 1),
		}

		t.registerMsrpAddr(req)
		t.MsrpConns[ck] = pconn

		pconn.br = bufio.NewReaderSize(pconn.conn, t.readBufferSize())
		pconn.bw = bufio.NewWriterSize(pconn.conn, t.writeBufferSize())

		go pconn.readLoop()
		go pconn.writeLoop()

		return pconn, nil
	}
}

func (pc *persistConn) close() {
	pc.conn.Close()
}

func (pc *persistConn) readLoop() {
	defer func() {
		delete(pc.t.MsrpConns, pc.cacheKey)
		delete(pc.t.MsrpAddrs, pc.addrKey)
		pc.close()
	}()

	alive := true
	for alive {
		mes, err := pc.readMessage()
		// 	v, err := pc.br.ReadString('\n')
		if err != nil {
			return
		}
		if mes.req != nil {
			req := mes.req
			fmt.Println(req.body)
			// change fromPath with toPath.
			pc.sendResponseOK(req.fromPath, req.toPath, req.tid)
			if req.isCpim {
				pc.sendHTTPCallBack(req.message)
			}
		} else if mes.res != nil {
			res := mes.res
			fmt.Println(res.body)
		} else {
			panic(err)
		}
	}
}

func (pc *persistConn) sendResponseOK(toPath, fromPath, tid string) {
	res, err := NewResponse(toPath, fromPath, tid)
	if err != nil {
		return
	}

	mes := Message{}
	mes.res = res
	fmt.Println(res.body)
	pc.writech <- mes
}

func (pc *persistConn) sendHTTPCallBack(message string) error {
	// TODO: make responseURL configurable
	responseURL := "http://localhost:3000/recieve?sender=" + pc.addrKey.sender + "&text=" + message
	fmt.Println(responseURL)

	_, err := http.Get(responseURL)

	return err
}

type Message struct {
	req *Request
	res *Response
}

func parseFirstLine(firstLine string) (tid string, isRequest bool, err error) {
	if err != nil {
		panic(err)
	}
	if strings.Contains(firstLine, "MSRP") {
		ss := strings.Split(firstLine, " ")
		if len(ss) < 3 {
			return "", false, errors.New("cannot read firstline")
		}
		tid = ss[1]
		if strings.Contains(firstLine, "SEND") {
			return tid, true, nil
		}
		return tid, false, nil
	}
	return "", false, errors.New("cannot read firstline")
}

func readRequest(b *bufio.Reader, tid string) (req Request, err error) {
	req = Request{}
	req.tid = tid
	var message string
	isPayload := false
	for true {
		l, err := b.ReadString('\n')
		if err != nil {
			return req, err
		}
		req.body += l
		l = strings.TrimSuffix(l, "\r\n")
		if strings.Contains(l, "-------") {
			break
		}
		if isPayload && l != "" {
			message += l
		} else if strings.Contains(l, "To-Path") {
			toPath := strings.Split(l, ": ")[1]
			req.toPath = toPath
		} else if strings.Contains(l, "From-Path") {
			fromPath := strings.Split(l, ": ")[1]
			req.fromPath = fromPath
		} else if strings.Contains(l, "Content-Length:") {
			isPayload = true
		} else if strings.Contains(l, "Content-Type: message/cpim") {
			req.isCpim = true
		} else if strings.Contains(l, "Content-type: message/imdn+xml") {
			req.isCpim = false
		}
	}
	req.message = message
	return req, nil
}

func readResponse(b *bufio.Reader, tid string) (res Response, err error) {
	res = Response{}
	res.tid = tid
	for true {
		l, err := b.ReadString('\n')
		if err != nil {
			return res, err
		}
		res.body += l
		l = strings.TrimSuffix(l, "\r\n")
		if strings.Contains(l, "-------") {
			break
		}
		if strings.Contains(l, "To-Path") {
			toPath := strings.Split(l, ": ")[1]
			res.toPath = toPath
		} else if strings.Contains(l, "From-Path") {
			fromPath := strings.Split(l, ": ")[1]
			res.fromPath = fromPath
		}
	}
	return res, nil
}

// Read both request and response
func (pc *persistConn) readMessage() (mes *Message, err error) {
	// Parse the first line of the message to check
	mes = &Message{}
	firstLine, err := pc.br.ReadString('\n')
	if err != nil {
		return mes, err
	}
	for len(firstLine) < 1 {
		firstLine, err = pc.br.ReadString('\n')
		if err != nil {
			return mes, err
		}
	}
	firstLine = strings.TrimSuffix(firstLine, "\r\n")
	tid, isReq, err := parseFirstLine(firstLine)
	if err != nil {
		return mes, err
	}
	if isReq {
		req, err := readRequest(pc.br, tid)
		req.body = firstLine + "\r\n" + req.body
		if err != nil {
			return mes, err
		}
		mes.req = &req
	} else {
		res, err := readResponse(pc.br, tid)
		res.body = firstLine + "\r\n" + res.body
		if err != nil {
			return mes, err
		}
		mes.res = &res
	}
	return mes, nil
}

func (pc *persistConn) writeLoop() {
	for {
		select {
		case mes := <-pc.writech:
			if mes.req != nil {
				err := mes.req.write(pc.bw)
				if err == nil {
					err = pc.bw.Flush()
				}
				if err != nil {
					pc.close()
					return
				}
			} else if mes.res != nil {
				err := mes.res.write(pc.bw)
				if err == nil {
					err = pc.bw.Flush()
				}
				if err != nil {
					pc.close()
					return
				}
			}
		}
	}
}
