// Package gote (go-telnet) provides a net.Conn compatible interface for connection to telnet servers
// that require telnet option negotiation. Behavior mimics net.Conn behavior wherever possible, exceptions noted.
package gote

import (
	"bytes"
	"fmt"
	"net"
	"sync"
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
	// Proposed methods
	// SetOption tries to set the option through negotiation with
	// the server.
	//SetOption(opt byte, val []byte) (success bool, err error)
	// RequestOption requests the status of an option from the server.
	//RequestOption(opt byte) (response []byte, err error)
}

// Con is the internal telnet connection object.
type conn struct {
	net.Conn
	quit      chan bool
	buf       [][]byte
	uLock     *sync.Mutex
	eLock     *sync.Mutex
	lastError error
	i         *bytes.Buffer // in from the connection
	u         *bytes.Buffer // upstream
}

// Dial connects to a TCP endpoint and returns a Telnet Connection object,
// which transparently handles telnet options and escaping.
func Dial(network, address string) (Connection, error) {
	fmt.Println("Dialing this: ", address)
	var t conn
	return t.dial(network, address)
}

// Dial is a helper function for creating and connecting to a telnet session.
func (c *conn) dial(network, address string) (Connection, error) {
	var err error
	c.Conn, err = net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	c.quit = make(chan bool, 1)
	c.uLock = &sync.Mutex{}
	c.eLock = &sync.Mutex{}
	//tcp input
	c.i = bytes.NewBuffer(nil)
	//upstream
	c.u = bytes.NewBuffer(nil)
	go c.process()
	return c, nil
}

// Read the current buffer sent from the server	fprint after being processed
// for telnet options. This blocks until data is available.
func (c *conn) Read(b []byte) (n int, err error) {
	// otherwise push the processed data
	c.uLock.Lock()
	defer c.uLock.Unlock()
	ready := c.u.Len() > 0
	for !ready {
		// push connection errors upstream, only after buffer has been sent
		c.eLock.Lock()
		if c.lastError != nil {
			return 0, c.lastError
		}
		c.eLock.Unlock()

		c.uLock.Unlock()
		time.Sleep(time.Duration(20) * time.Millisecond)
		c.uLock.Lock()
		ready = c.u.Len() > 0
	}
	return c.u.Read(b)
}

// Write the byte buffer to the output stream. Escaping 255 bytes is done
// automatically, so is not required by the caller. Note that the written
// count may be off due to the 255 byte escaping. This will be fixed in future releases.
// Currently not thread safe, although that functionality may be added later.
func (c *conn) Write(b []byte) (n int, err error) {
	l1 := len(b)
	for i := 0; i < l1; i++ {
		// If the stream contains a 255, then escape it by sending a second 255
		if b[i] == IAC {
			b = append(b, 0)
			copy(b[i+1:], b[i:])
			b[i] = byte(255)
		}
	}

	_, err = c.write(b)
	// TODO: Calculate the deltas of what was written vs expected to calculate "upstream/assumed" written bytes
	return l1, err
}

func (c *conn) write(b []byte) (n int64, err error) {
	c.buf = append(c.buf, b)
	return (*net.Buffers)(&c.buf).WriteTo(c.Conn)
}

// Close the connection
// This is a pass-through method to the underlying net.conn
// without any processing.
func (c *conn) Close() error {
	c.quit <- true
	return c.Conn.Close()
}

// Buffer reads from the underlying TCP connection and buffers as necessary,
// passing it onto process to handle Telnet commands.
func (c *conn) buffer(quit chan bool, updates chan []byte, errors chan error) {
	buf := make([]byte, 2048)
	for {
		i, err := c.Conn.Read(buf)
		if err != nil {
			errors <- err
		}
		if i > 0 {
			updates <- buf[:i]
		} else {
			time.Sleep(time.Duration(30) * time.Millisecond)
		}
		select {
		case <-quit:
			break
		default:
		}
	}
}

// Process parses the buffer for telnet IAC commands,
// and forwards on the results either upstream or to be handled as a telnet command.
func (c *conn) process() {
	bufquit := make(chan bool, 1)
	updates := make(chan []byte, 1024)
	errors := make(chan error, 2)

	go c.buffer(bufquit, updates, errors)

	for {
		toProcess := c.i.Len() > 0
		if toProcess {
			b := c.i.Bytes()
			c.uLock.Lock()
			//If no 255's exist, just copy and move on
			if i := bytes.IndexByte(b, IAC); i == -1 {
				c.i.WriteTo(c.u)
			} else {
				//handle the IAC here
				//read from the input process up to, but not including, the 255
				c.u.Write(c.i.Next(i))
				c.processIAC()
			}
			c.uLock.Unlock()
		}
		select {
		case <-c.quit:
			bufquit <- true
			return
		case b := <-updates:
			c.i.Write(b)
		case err := <-errors:
			c.eLock.Lock()
			c.lastError = err
			c.eLock.Unlock()
		default:
		}
		// If the input process is empty, that means the connection is also empty so let's wait a bit
		if !toProcess {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
	}
}

// ProcessIAC determines if the IAC is an escaped 255 byte,
// or an actual command to be processed. If it's an escaped byte, it removes
// the duplication/escaping and forwards the buffer upstream.
func (c *conn) processIAC() {
	// If there is only a single character, don't process since we can't do anything with it
	if c.i.Len() <= 1 {
		return
	}
	b := c.i.Bytes()
	// If this is an escaped 255, write a single 255 to the output process and move the
	// pointer forwards twice
	if b[0] == 255 && b[1] == 255 {
		c.u.Write(c.i.Next(1))
		_ = c.i.Next(1)
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
		c.Conn.Write([]byte{255, DO, SGA})
	default:
		c.Conn.Write([]byte{255, DONT, opt})
	}
	// consume IAC, Cmd, and Option from the input process
	_ = c.i.Next(3)
}

// Dont responds to Telnet DONT commands.
// By default it accepts all DONT commands and responds with WONT <opt>
func (c *conn) dont(buf []byte) {
	// if we don't have the option in the process yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	c.Conn.Write([]byte{255, WONT, opt})
	// consume IAC, Cmd, and Option from the input process
	_ = c.i.Next(3)
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
		c.Conn.Write([]byte{255, WILL, BIN})
		break
	default:
		c.Conn.Write([]byte{255, WONT, opt})
	}
	// consume IAC, Cmd, and Option from the input process
	c.i.Next(3)
}

// Wont responds to Telnet WONT commands.
// By default it consumes these commands without any further processing.
func (c *conn) wont(buf []byte) {
	// if we don't have the option in the process yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	// consume IAC, Cmd, and Option from the input process
	_ = c.i.Next(3)
}
