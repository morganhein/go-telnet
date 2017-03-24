// Package gote (go-telnet) provides a net.Conn compatible interface for connection to telnet servers
// that require telnet option negotiation
package gote

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

// Options
const (
	BIN  = byte(0) // Binary Transmission
	ECHO = byte(1)
	REC  = byte(2)  // Reconnect
	SGA  = byte(3)  // Suppress Go Ahead
	LOG  = byte(18) // Logout
	TSP  = byte(32) // Terminal Speed
	RFC  = byte(33) // Remote Flow Control
)

// Connection is a telnet interface which implements net.conn, along
// with some proposed extended functionality for handling telnet options.
type Connection interface {
	// Read the data sent from the server after being processed
	// for telnet options.
	Read(b []byte) (n int, err error)
	// Write the byte buffer to the output stream. Escaping 255 bytes is done
	// automatically, so is not required by the caller. Note that the written
	// count may be off due to the 255 byte escaping.
	Write(b []byte) (n int, err error)
	// Close the connection
	// This is a pass-through method to the underlying net.conn
	// without any processing.
	Close() error
	// LocalAddr returns the LocalAddress of this connection.
	// This is a pass-through method to the underlying net.conn
	// without any processing.
	LocalAddr() net.Addr
	// RemoteAddr returns the RemoteAddress of this connection.
	// This is a pass-through method to the underlying net.conn
	// without any processing.
	RemoteAddr() net.Addr
	// SetDeadline is a pass-through method to the underlying net.conn
	// without any processing.
	SetDeadline(t time.Time) error
	// SetReadDeadline is a pass-through method to the underlying net.conn
	// without any processing.
	SetReadDeadline(t time.Time) error
	// SetWriteDeadline is a pass-through method to the underlying net.conn
	// without any processing.
	SetWriteDeadline(t time.Time) error
	//SetOption(opt byte, val []byte) (success bool, err error) proposed for future development.
}

// Con is the internal telnet connection object.
type conn struct {
	c    net.Conn
	quit chan bool
	bIn  *bytes.Buffer // in from the connection
	bOut *bytes.Buffer // upstream
}

// Dial connects to a TCP endpoint and returns a Telnet Connection object,
// which transparently handles telnet options and escaping.
func Dial(network, address string) (Connection, error) {
	fmt.Println("Connecting.")
	var t conn
	return t.dial(network, address)
}

// Dial is a helper function for creating and connecting to a telnet session.
func (c *conn) dial(network, address string) (Connection, error) {
	var err error
	c.c, err = net.Dial(network, address)
	c.quit = make(chan bool, 1)
	go c.process()
	return c, err
}

// Read the current process sent from the server after being processed
// for telnet options.
func (c *conn) Read(b []byte) (n int, err error) {
	return c.bOut.Read(b)
}

// Process buffers incoming traffic, parses it for telnet IAC commands,
// and forwards on the results either upstream or to be handled as a telnet command.
func (c *conn) process() {
	//bIn process from the underlying TCP connection
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
				//read from the input process up to, but not including, the 255
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

		// If the input process is empty, that means the connection is also empty so let's wait a bit
		if c.bIn.Len() == 0 {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
	}
}

// ProcessIAC determines if the IAC is an escaped 255 byte,
// or an actual command to be processed. If it's an escaped byte, it removes
// the duplication/escaping and forwards the buffer upstream.
func (c *conn) processIAC() {
	// If there is only a single character, don't process since we can't do anything with it
	if c.bIn.Len() <= 1 {
		return
	}
	b := c.bIn.Bytes()
	// If this is an escaped 255, write a single 255 to the output process and move the
	// pointer forwards twice
	if b[0] == 255 && b[1] == 255 {
		c.bOut.Write(c.bIn.Next(1))
		_ = c.bIn.Next(1)
		return
	}
	c.parseCommand(b)
}

// ParseCommand is a simple switch to figure out what command this is,
// and forward it on for processing.
func (c *conn) parseCommand(buff []byte) {
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

// Will responds to Telnet WILL commands.
// By default it enables Stop-Go-Ahead, and refuses everything else.
func (c *conn) will(buf []byte) {
	// if we don't have the option in the process yet, return and wait for more information
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
	// consume IAC, Cmd, and Option from the input process
	_ = c.bIn.Next(3)
}

// Dont responds to Telnet DONT commands.
// By default it accepts all DONT commands and responds with WONT <opt>
func (c *conn) dont(buf []byte) {
	// if we don't have the option in the process yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	c.c.Write([]byte{255, WONT, opt})
	// consume IAC, Cmd, and Option from the input process
	_ = c.bIn.Next(3)
}

// Do responds to Telnet DO commands.
// By default it accepts Binary transmissions, and refuses all other options.
func (c *conn) do(buf []byte) {
	// if we don't have the option in the process yet, return and wait for more information
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
	// consume IAC, Cmd, and Option from the input process
	c.bIn.Next(3)
}

// Wont responds to Telnet WONT commands.
// By default it consumes these commands without any further processing.
func (c *conn) wont(buf []byte) {
	// if we don't have the option in the process yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	// consume IAC, Cmd, and Option from the input process
	_ = c.bIn.Next(3)
}

// Write the byte process to the output stream. Escaping 255 bytes is done
// automatically, so is not required by the caller. Note that the written
// count may be off due to the 255 byte escaping.
func (c *conn) Write(b []byte) (n int, err error) {
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

// Close the connection
// This is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) Close() error {
	c.quit <- true
	return c.c.Close()
}

// LocalAddr returns the LocalAddress of this connection.
// This is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

// RemoteAddr returns the RemoteAddress of this connection.
// This is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

// SetDeadline is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

// SetReadDeadline is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

// SetWriteDeadline is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}
