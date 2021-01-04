package msrp

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWriteRequest(t *testing.T) {
	// Request
	var buf bytes.Buffer
	req, _ := NewRequest("msrp://localhost:9670/pxd512029144298;tcp", "msrp://localhost:8881/71f1vpJTi3rhgHUHj;tcp", "+8210", "+8211", "Hello!")
	req.write(&buf)
	fmt.Println(buf.String())

	req, _ = NewRequest("msrp://localhost:9670/pxd512029144298;tcp", "msrp://localhost:8881/71f1vpJTi3rhgHUHj;tcp", "+8210", "+8211", "")
	buf.Reset()
	req.write(&buf)
	fmt.Println(buf.String())
}
