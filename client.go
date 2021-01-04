package msrp

type Client struct {
	Transport   *Transport
	ResponseURL string
}

// DefaultClient is the default Client.
var DefaultClient = &Client{}

func (c *Client) Do(req *Request) (err error) {
	// Send message.
	err = c.send(req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) send(req *Request) (err error) {
	// Establish connection and return responses.
	err = send(req, c.transport())
	return err
}

func send(req *Request, t *Transport) (err error) {
	err = t.OneWay(req)
	return err
}

func (c *Client) transport() *Transport {
	if c.Transport != nil {
		return c.Transport
	}
	return DefaultTransport
}

func (c *Client) GetConnectionMethod(sender, recipient string) *connectMethod {
	t := c.transport()
	return t.GetConnectionMethod(sender, recipient)
}
