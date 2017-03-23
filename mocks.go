package go_telnet

import (
	"bytes"
	"net"
	"time"
)

// Mocks

type mockConn struct {
	c *bytes.Buffer
}

func (m mockConn) Read(b []byte) (n int, err error) {
	return m.c.Read(b)
}

func (m mockConn) Write(b []byte) (n int, err error) {
	return m.c.Write(b)
}

func (mockConn) Close() error {
	return nil
}

func (mockConn) LocalAddr() net.Addr {
	return Addr{
		NetworkString: "tcp",
		AddrString:    "127.0.0.1",
	}
}

func (mockConn) RemoteAddr() net.Addr {
	return Addr{
		NetworkString: "tcp",
		AddrString:    "127.0.0.1",
	}
}

func (mockConn) SetDeadline(t time.Time) error {
	return nil

}

func (mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type Addr struct {
	NetworkString string
	AddrString    string
}

func (a Addr) Network() string {
	return a.NetworkString
}

func (a Addr) String() string {
	return a.AddrString
}
