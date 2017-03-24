package go_telnet

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"
)

// Commands
const (
	IAC  = byte(255)
	DONT = byte(254)
	DO   = byte(253)
	WONT = byte(252)
	WILL = byte(251)
	SB   = byte(250) // Sub Negotiation
	GA   = byte(249) // Go Ahead
	EL   = byte(248) // Erase Line
	EC   = byte(247) // Erase Character
	AYT  = byte(246) // Are You There
	AO   = byte(245) // Abort Operation
	IP   = byte(244) // Interrupt Process
	BRK  = byte(243) // Break
	NOP  = byte(241) // No operation
	SE   = byte(240) // End of Subnegotiation
)

const (
	BIN  = byte(0) // Binary Transmission
	ECHO = byte(1)
	REC  = byte(2)  // Reconnect
	SGA  = byte(3)  // Suppress Go Ahead
	LOG  = byte(18) // Logout
	TSP  = byte(32) // Terminal Speed
	RFC  = byte(33) // Remote Flow Control
)

type Connection struct {
	c    net.Conn
	quit chan bool
	bIn  *bytes.Buffer // in from the connection
	bOut *bytes.Buffer // upstream
}

func Dial(network, address string) (net.Conn, error) {
	fmt.Println("Connecting.")
	var t Connection
	return t.Dial(network, address)
}

func (c *Connection) Dial(network, address string) (*Connection, error) {
	var err error
	c.c, err = net.Dial(network, address)
	c.quit = make(chan bool, 1)
	go c.buffer()
	return c, err
}

func (c *Connection) Read(b []byte) (n int, err error) {
	return c.bOut.Read(b)
}

func (c *Connection) buffer() {
	//bIn buffer from the underlying TCP connection
	c.bIn = bytes.NewBuffer(nil)
	//bOUt goes upstream
	c.bOut = bytes.NewBuffer(nil)

	go io.Copy(c.bIn, c.c)

	for {
		// if there's data to process
		if b := c.bIn.Bytes(); len(b) > 0 {
			//If no 255's exist, just copy and move on
			if i := bytes.IndexByte(b, IAC); i == -1 {
				c.bIn.WriteTo(c.bOut)
			} else {
				//handle the IAC here
				//read from the input buffer up to, but not including, the 255
				c.bOut.Write(c.bIn.Next(i))
				c.processIAC()
			}
		}

		select {
		case <-c.quit:
			return
		default:
			break
		}

		// If the input buffer is empty, that means the connection is also empty so let's wait a bit
		if c.bIn.Len() == 0 {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
	}
}

func (c *Connection) processIAC() {
	// If there is only a single character, don't process since we can't do anything with it
	if c.bIn.Len() <= 1 {
		return
	}
	b := c.bIn.Bytes()
	// If this is an escaped 255, write a single 255 to the output buffer and move the
	// pointer forwards twice
	if b[0] == 255 && b[1] == 255 {
		c.bOut.Write(c.bIn.Next(1))
		_ = c.bIn.Next(1)
		return
	}
	c.parseCommand(b)
}

func (c *Connection) parseCommand(buff []byte) {
	// iac := buff[0]
	cmd := buff[1]
	switch cmd {
	case DONT:
		c.dont(buff)
	case DO:
		c.do(buff)
	case WONT:
		c.wont(buff)
	case WILL:
		c.will(buff)
	//case SB:
	//	break
	//case AYT:
	//	break
	//case NOP:
	//	break
	//case SE:
	//	break
	default:
		break
	}
}

func (c *Connection) will(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	switch opt {
	case SGA:
		c.c.Write([]byte{255, DO, SGA})
	default:
		c.c.Write([]byte{255, DONT, opt})
	}
	// consume IAC, Cmd, and Option from the input buffer
	_ = c.bIn.Next(3)
}

func (c *Connection) dont(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	c.c.Write([]byte{255, WONT, opt})
	// consume IAC, Cmd, and Option from the input buffer
	_ = c.bIn.Next(3)
}

func (c *Connection) do(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	switch opt {
	case BIN:
		c.c.Write([]byte{255, WILL, BIN})
		break
	default:
		c.c.Write([]byte{255, WONT, opt})
	}
	// consume IAC, Cmd, and Option from the input buffer
	c.bIn.Next(3)
}

func (c *Connection) wont(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	// consume IAC, Cmd, and Option from the input buffer
	_ = c.bIn.Next(3)
}

func (c *Connection) Write(b []byte) (n int, err error) {
	for i := 0; i < len(b); i++ {
		// If the stream contains a 255, then escape it by sending a second 255
		if b[i] == IAC {
			b = append(b, 0)
			copy(b[i+1:], b[i:])
			b[i] = byte(255)
		}
	}
	//Not abstracting away the duplicate 255 bytes that might have been sent
	//Todo: possibly return the unescaped byte count instead
	return c.c.Write(b)
}

func (c *Connection) Close() error {
	c.quit <- true
	return c.c.Close()
}

func (c *Connection) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *Connection) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

func (c *Connection) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}
