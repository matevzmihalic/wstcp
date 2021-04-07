package wstcp

import (
	"bytes"
	"io"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// WSTCP implements io.ReadWriteCloser
type WSTCP struct {
	conn       io.ReadWriteCloser
	firstBytes []byte
	remaining  int

	wsReader *wsutil.Reader
	wsWriter *wsutil.Writer
}

// New wraps incoming connection in WSTCP
func New(conn io.ReadWriteCloser) (*WSTCP, error) {
	out := &WSTCP{
		conn:       conn,
		firstBytes: make([]byte, 3),
	}

	_, err := io.ReadFull(conn, out.firstBytes)
	if err != nil {
		return nil, err
	}

	if bytes.Equal(bytes.ToLower(out.firstBytes), []byte("get")) {
		_, err = ws.Upgrade(out)
		if err != nil {
			return nil, err
		}

		state := ws.StateServerSide
		out.wsReader = &wsutil.Reader{
			Source:         conn,
			State:          state,
			CheckUTF8:      true,
			OnIntermediate: wsutil.ControlFrameHandler(conn, state),
		}
		out.wsWriter = wsutil.NewWriter(conn, state, 0)
	}

	return out, nil
}

func (c *WSTCP) Read(b []byte) (int, error) {
	if c.wsReader == nil {
		return c.read(b)
	}

	if c.remaining != 0 {
		return c.readWS(b, c.remaining)
	}

	h, err := c.wsReader.NextFrame()
	if err != nil {
		return 0, err
	}

	if h.OpCode.IsControl() {
		if !c.wsReader.State.Fragmented() {
			err := c.wsReader.OnIntermediate(h, c.wsReader)
			if _, isClosed := err.(wsutil.ClosedError); isClosed {
				return 0, io.EOF
			}
			if err != nil {
				return 0, err
			}
		}
		return c.Read(b)
	}

	if h.OpCode == ws.OpText || h.OpCode == ws.OpBinary {
		c.wsWriter.Reset(c.conn, c.wsReader.State, h.OpCode)
	}

	n, err := c.readWS(b, int(h.Length))
	if err != nil {
		return n, err
	}

	if !h.Fin && n < len(b) {
		n2, err := c.Read(b[n:])
		return n + n2, err
	}

	return n, nil
}

func (c *WSTCP) readWS(b []byte, dataLen int) (int, error) {
	max := dataLen
	if max > len(b) {
		max = len(b)
	}

	n, err := io.ReadFull(c.wsReader, b[:max])
	if err == io.EOF {
		err = nil
	}

	c.remaining = dataLen - n

	return n, err
}

func (c *WSTCP) read(b []byte) (int, error) {
	if c.firstBytes == nil {
		return c.conn.Read(b)
	}

	n := copy(b, c.firstBytes)

	if n < len(c.firstBytes) {
		c.firstBytes = c.firstBytes[n:]
		return n, nil
	}

	c.firstBytes = nil
	n2, err := c.conn.Read(b[n:])

	return n + n2, err
}

func (c *WSTCP) Write(b []byte) (int, error) {
	if c.wsWriter == nil {
		return c.conn.Write(b)
	}

	n, err := c.wsWriter.Write(b)
	if err != nil {
		return n, err
	}

	return n, c.wsWriter.Flush()
}

func (c *WSTCP) Close() error {
	return c.conn.Close()
}
