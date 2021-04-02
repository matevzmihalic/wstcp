# WebSocket | TCP

is a Go library for serving WebSocket or TCP clients on the same listener.
It can be used as a drop in replacement to add WebSocket support to existing TCP servers.

It automatically detects WebSocket clients, upgrades them and then takes care of headers and control packets.

## Usage

```go
ln, _ := net.Listen("tcp", "localhost:9999")
for {
    rawConn, _ := ln.Accept()
    go func() {
        conn, _ := wstcp.New(rawConn)
        // conn can now be used the same way rawConn would be
        // but it can handle WebSocket or raw TCP clients
    }()
}
```

