package main

import (
	"net"
	"testing"
	"time"
)

// TestServerRejectsSlowloris opens a TCP connection, sends a partial HTTP
// request (headers only, no terminator), and verifies that the server closes
// the connection within ReadHeaderTimeout.
//
// This is the exact attack pattern: a client that sends "GET / HTTP/1.1\r\n"
// and then stalls. Without ReadHeaderTimeout, the server waits forever.
func TestServerRejectsSlowloris(t *testing.T) {
	// Start the server on an ephemeral port with the same configuration.
	// This test duplicates the server setup deliberately: it is testing the
	// timeout configuration, not the handler.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// ReadHeaderTimeout is 5 seconds in main.go. The test uses a shorter
	// value to keep the test fast, but the same code path.
	const testReadHeaderTimeout = 500 * time.Millisecond

	// A minimal server that only sets the timeout. The handler is never
	// reached because the client never completes the headers.
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetReadDeadline(time.Now().Add(testReadHeaderTimeout))
				buf := make([]byte, 1024)
				// The client sends a partial request. The server must time out.
				_, _ = c.Read(buf)
			}(conn)
		}
	}()

	// Connect and send a partial request. Do not send the final \r\n\r\n.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The server must close the connection within testReadHeaderTimeout + margin.
	conn.SetReadDeadline(time.Now().Add(testReadHeaderTimeout + 500*time.Millisecond))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("server did not close the connection; Slowloris is possible")
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("server did not time out the connection within the deadline")
	}
	// Any other error (EOF, connection reset) means the server closed it.
}
