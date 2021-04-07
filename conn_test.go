package wstcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/stretchr/testify/assert"
)

func TestWSTCP_integration(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:")
	assert.NoError(t, err)

	go func() {
		for {
			inConn, err := ln.Accept()
			if errors.Is(err, net.ErrClosed) {
				break
			}
			assert.NoError(t, err)

			go func() {
				conn, err := New(inConn)
				assert.NoError(t, err)
				defer inConn.Close()

				const buffLen = 3
				b := make([]byte, buffLen)
				for {
					fullLen := 0
					for {
						n, err := conn.Read(b[fullLen:])
						fullLen += n
						if err != nil {
							return
						}
						if n != buffLen {
							break
						}
						b = append(b, make([]byte, buffLen)...)
					}

					_, err = conn.Write(b[:fullLen])
					assert.NoError(t, err)
				}
			}()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		c, _, _, err := ws.Dial(context.Background(), "ws://"+ln.Addr().String())
		assert.NoError(t, err)

		go func() {
			out := make([]byte, 0, 2)
			for {
				b, err := wsutil.ReadServerBinary(c)
				out = append(out, b...)
				if err != nil {
					break
				}
			}

			assert.Equal(t, []byte("12346789000"), out)
			assert.NoError(t, c.Close())
			wg.Done()
		}()

		err = wsutil.WriteClientBinary(c, []byte("1234"))
		assert.NoError(t, err)

		err = wsutil.WriteClientBinary(c, []byte("6789000"))
		assert.NoError(t, err)

		err = ws.WriteHeader(c, ws.Header{
			Fin:    true,
			Masked: true,
			OpCode: ws.OpClose,
		})
		assert.NoError(t, err)
	}()

	go func() {
		c, err := net.Dial("tcp", ln.Addr().String())
		assert.NoError(t, err)

		go func() {
			b := make([]byte, 11)
			n, err := io.ReadFull(c, b)
			assert.NoError(t, err)
			assert.Equal(t, 11, n)
			assert.Equal(t, []byte("12346789000"), b)
			assert.NoError(t, c.Close())
			wg.Done()
		}()

		_, err = c.Write([]byte("1234"))
		assert.NoError(t, err)

		_, err = c.Write([]byte("6789000"))
		assert.NoError(t, err)
	}()

	wg.Wait()
	ln.Close()
}

type mockRWC struct {
	bytes.Buffer
	closed    bool
	closedErr error
	writeErr  error
}

func (m *mockRWC) Close() error {
	m.closed = true
	return m.closedErr
}

func (m *mockRWC) Write(b []byte) (int, error) {
	n, err := m.Buffer.Write(b)
	if err != nil {
		return n, err
	}

	return n, m.writeErr
}
