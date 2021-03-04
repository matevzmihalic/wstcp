package wstcp

import (
	"bytes"
	"io"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WSTCP struct {
	rwc        io.ReadWriteCloser
	firstBytes []byte

	wsReader *wsutil.Reader
	wsWriter *wsutil.Writer
}

func New(rwc io.ReadWriteCloser) (*WSTCP, error) {
	out := &WSTCP{
		rwc:        rwc,
		firstBytes: make([]byte, 3),
	}

	_, err := io.ReadFull(rwc, out.firstBytes)
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
			Source:         rwc,
			State:          state,
			CheckUTF8:      true,
			OnIntermediate: wsutil.ControlFrameHandler(rwc, state),
		}
		out.wsWriter = wsutil.NewWriter(rwc, state, 0)
	}

	return out, nil
}

func (c *WSTCP) Read(b []byte) (int, error) {
	if c.wsReader == nil {
		return c.read(b)
	}

	h, err := c.wsReader.NextFrame()
	if err != nil {
		return 0, err
	}

	if h.OpCode.IsControl() {
		if !c.wsReader.State.Fragmented() {
			err := c.wsReader.OnIntermediate(h, c.wsReader)
			if err != nil {
				return 0, err
			}
		}
		return c.Read(b)
	}

	if h.OpCode == ws.OpText || h.OpCode == ws.OpBinary {
		c.wsWriter.Reset(c.rwc, c.wsReader.State, h.OpCode)
	}

	maxLen := int(h.Length)
	if maxLen > len(b) {
		maxLen = len(b)
	}

	n, err := io.ReadFull(c.wsReader, b[:maxLen])
	if err == io.EOF {
		err = nil
	}

	if !h.Fin {
		n2, err := c.Read(b[n:])
		return n + n2, err
	}

	return n, err
}

func (c *WSTCP) read(b []byte) (int, error) {
	if c.firstBytes == nil {
		return c.rwc.Read(b)
	}

	n := copy(b, c.firstBytes)

	if n < len(c.firstBytes) {
		c.firstBytes = c.firstBytes[n:]
		return n, nil
	}

	c.firstBytes = nil
	n2, err := c.rwc.Read(b[n:])

	return n + n2, err
}

func (c *WSTCP) Write(b []byte) (int, error) {
	if c.wsWriter == nil {
		return c.rwc.Write(b)
	}

	n, err := c.wsWriter.Write(b)
	if err != nil {
		return n, err
	}

	return n, c.wsWriter.Flush()
}

func (c *WSTCP) Close() error {
	return c.rwc.Close()
}
