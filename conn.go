package wstcp

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type WSTCP struct {
	rwc        io.ReadWriteCloser
	firstBytes []byte

	ws       bool
	wsReader *wsutil.Reader
	binary   bool
	length   int
}

var (
	ErrOpTextNotSupported = fmt.Errorf("op text not supported")
	ErrIncorrectRSV       = fmt.Errorf("incorrect RSV")
)

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
		out.ws = true
		out.wsReader = wsutil.NewServerSideReader(rwc)
	}

	return out, nil
}

func (c *WSTCP) Read(b []byte) (int, error) {
	if !c.ws {
		return c.read(b)
	}

	fin := true
	if c.length == 0 {
		header, err := c.wsReader.NextFrame()
		//if err == ws.ErrProtocolNonZeroRsv {
		//	return 0, c.rwc.Close()
		//} else
		if err != nil {
			return 0, err
		}
		fin = header.Fin

		switch header.OpCode {
		case ws.OpPing:
			header.OpCode = ws.OpPong
			header.Masked = false
			err := ws.WriteHeader(c.rwc, header)
			if err != nil {
				return 0, err
			}
			_, err = io.CopyN(c.rwc, c.wsReader, header.Length)
			if err != nil {
				return 0, err
			}

			return c.Read(b)

		case ws.OpPong:
			_, err := io.CopyN(ioutil.Discard, c.wsReader, header.Length)
			if err != nil {
				return 0, err
			}
			return c.Read(b)

		case ws.OpClose:
			if err := c.Close(); err != nil {
				return 0, err
			}
			return c.Read(b)

		case ws.OpBinary:
			c.binary = true

		case ws.OpText:
			c.binary = false
		}

		c.length = int(header.Length)
	}

	maxLen := c.length
	if maxLen > len(b) {
		maxLen = len(b)
	}
	n, err := io.ReadFull(c.wsReader, b[:maxLen])
	//n, err := c.wsReader.Read(b)
	fmt.Println(fin)
	if !fin {
		n2, err := c.Read(b[n:])
		return n + n2, err
	}
	if err == io.EOF {
		return c.Read(b)
	}
	//if err == io.EOF {
	//	return c.Read(b)
	//
	//}

	c.length -= n

	return n, err
	//
	//if c.length == 0 {
	//	err := c.handleWSHeader()
	//	if err != nil {
	//		return 0, err
	//	}
	//}
	//
	//n = c.length
	//if n > len(b) {
	//	n = len(b)
	//}
	//
	//n, err = c.rwc.Read(b[:n])
	//if err != nil {
	//	return 0, err
	//}
	//ws.Cipher(b[:n], c.mask, c.pos)
	//c.pos += n
	//c.length -= n
	//
	//return n, nil
}

//func (c *WSTCP) handleWSHeader() error {
//	header, err := ws.ReadHeader(c.rwc)
//	if err != nil {
//		return err
//	}
//
//	switch header.OpCode {
//	case ws.OpClose:
//		err = c.Close()
//		if err != nil {
//			return err
//		}
//		return io.EOF
//
//	case ws.OpPing:
//		err = ws.WriteHeader(c.rwc, ws.Header{
//			OpCode: ws.OpPong,
//		})
//		return err
//
//	case ws.OpText:
//		return fmt.Errorf("OpText not supported")
//	}
//
//	if !header.Masked {
//		return fmt.Errorf("must be masked")
//	}
//
//	c.length = int(header.Length)
//	c.mask = header.Mask
//	c.pos = 0
//
//	return nil
//}

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
	if !c.ws {
		return c.rwc.Write(b)
	}

	var err error
	if c.binary {
		err = wsutil.WriteServerBinary(c.rwc, b)
	} else {
		err = wsutil.WriteServerText(c.rwc, b)
	}
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *WSTCP) Close() error {
	if !c.ws {
		return c.rwc.Close()
	}

	err := ws.WriteFrame(c.rwc, ws.NewCloseFrame(nil))
	if err != nil {
		c.rwc.Close()
		return err
	}

	return c.rwc.Close()
}
