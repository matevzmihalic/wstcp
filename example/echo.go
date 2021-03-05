package main

import (
	"flag"
	"log"
	"net"

	"github.com/matevzmihalic/wstcp"
)

func main() {
	address := flag.String("address", "0.0.0.0:9099", "Address for the server to listen to")
	flag.Parse()

	ln, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Started server on", ln.Addr().String())

	for {
		inConn, err := ln.Accept()
		if err != nil {
			log.Println("ln.Accept", err)
			break
		}
		log.Println("Accepted new connection", inConn.RemoteAddr().String())

		go func() {
			conn, err := wstcp.New(inConn)
			if err != nil {
				log.Println("wstcp.New", err)
				return
			}
			defer conn.Close()

			b := make([]byte, 66000)
			for {
				n, err := conn.Read(b)
				if err != nil {
					log.Println("conn.Read", err)
					return
				}

				_, err = conn.Write(b[:n])
				if err != nil {
					log.Println("conn.Write", err)
					return
				}
			}
		}()
	}
}
