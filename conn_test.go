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

				b := make([]byte, 3)
				for {
					n, err := conn.Read(b)
					if err != nil {
						return
					}

					_, err = conn.Write(b[:n])
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
			out := make([]byte, 0, 11)
			for {
				b, err := wsutil.ReadServerBinary(c)
				out = append(out, b...)
				if err != nil || len(out) == 11 {
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

func TestWSTCP_Close(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		connection      WSTCP
		rwc             *mockRWC
		expected        error
		expectedWritten []byte
	}{
		{
			name: "non ws success",
			connection: WSTCP{
				ws: false,
			},
			rwc: &mockRWC{},
		},
		{
			name: "non ws with error",
			connection: WSTCP{
				ws: false,
			},
			rwc: &mockRWC{
				closedErr: io.ErrClosedPipe,
			},
			expected: io.ErrClosedPipe,
		},
		{
			name: "ws err on close",
			connection: WSTCP{
				ws: true,
			},
			rwc: &mockRWC{
				closedErr: io.ErrClosedPipe,
			},
			expected: io.ErrClosedPipe,
		},
		{
			name: "ws err on write",
			connection: WSTCP{
				ws: true,
			},
			rwc: &mockRWC{
				writeErr: io.EOF,
			},
			expected: io.EOF,
		},
		{
			name: "ws success",
			connection: WSTCP{
				ws: true,
			},
			rwc:             &mockRWC{},
			expectedWritten: []byte{0x88, 0x0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc

			tc.connection.rwc = tc.rwc
			err := tc.connection.Close()
			if tc.expected == nil {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedWritten, tc.rwc.Bytes())
			} else {
				assert.EqualError(t, err, tc.expected.Error())
			}
			assert.True(t, tc.rwc.closed)
		})
	}
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
