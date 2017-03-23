package go_telnet

import (
	"bufio"
	"bytes"
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

type Telnet struct {
	c    net.Conn
	quit chan bool
	bIn  *bytes.Buffer // in from the connection
	bOut *bytes.Buffer // upstream
}

func Dial(network, address string) (net.Conn, error) {
	var t Telnet
	return t.Dial(network, address)
}

func (te *Telnet) Dial(network, address string) (*Telnet, error) {
	var err error
	te.c, err = net.Dial(network, address)
	te.quit = make(chan bool, 1)
	go te.buffer()
	return te, err
}

func (te *Telnet) Read(b []byte) (n int, err error) {
	return te.bOut.Read(b)
}

func (te *Telnet) buffer() {
	te.bIn = bytes.NewBuffer(nil)
	te.bOut = bytes.NewBuffer(nil)
	connBuff := bufio.NewReader(te.c)
	for {
		b := te.bIn.Bytes()
		//If no 255's exist, just copy and move on
		if i := bytes.IndexByte(b, IAC); i == -1 {
			te.bOut.WriteTo(te.bOut)
		} else {
			//handle the IAC here
			//read from the input buffer up to, but not including, the 255
			te.bOut.Write(te.bIn.Next(i))
			te.processIAC()
		}
		select {
		case <-te.quit:
			return
		default:
			break
		}
		//refill the input buffer
		connBuff.Peek(1)
		if connBuff.Buffered() > 0 {
			connBuff.WriteTo(te.bIn)
		}
		// If the input buffer is empty, that means the connection is also empty so let's wait a bit
		if te.bIn.Len() == 0 {
			time.Sleep(time.Duration(100) * time.Millisecond)
		}
	}
}

func (te *Telnet) processIAC() {
	// If there is only a single character, don't process since we can't do anything with it
	if te.bIn.Len() <= 1 {
		return
	}
	b := te.bIn.Bytes()
	// If this is an escaped 255, write a single 255 to the output buffer and move the
	// pointer forwards twice
	if b[0] == 255 && b[1] == 255 {
		te.bOut.Write(te.bIn.Next(1))
		_ = te.bIn.Next(1)
		return
	}
	te.parseCommand(b)
}

func (te *Telnet) parseCommand(buff []byte) {
	// iac := buff[0]
	cmd := buff[1]
	switch cmd {
	case DONT:
		te.dont(buff)
	case DO:
		te.do(buff)
	case WONT:
		te.wont(buff)
	case WILL:
		te.will(buff)
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

func (te *Telnet) will(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	te.c.Write([]byte{255, DONT, opt})
	// consume IAC, Cmd, and Option from the input buffer
	_ = te.bIn.Next(3)
}

func (te *Telnet) dont(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	te.c.Write([]byte{255, WONT, opt})
	// consume IAC, Cmd, and Option from the input buffer
	_ = te.bIn.Next(3)
}

func (te *Telnet) do(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	opt := buf[2]
	switch opt {
	case BIN:
		te.c.Write([]byte{255, WILL, BIN})
		break
	default:
		te.c.Write([]byte{255, WONT, opt})
	}
	// consume IAC, Cmd, and Option from the input buffer
	_ = te.bIn.Next(3)
}

func (te *Telnet) wont(buf []byte) {
	// if we don't have the option in the buffer yet, return and wait for more information
	if len(buf) < 3 {
		return
	}
	// consume IAC, Cmd, and Option from the input buffer
	_ = te.bIn.Next(3)
}

func (te *Telnet) Write(b []byte) (n int, err error) {
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
	return te.c.Write(b)
}

func (te *Telnet) Close() error {
	te.quit <- true
	return te.c.Close()
}

func (te *Telnet) LocalAddr() net.Addr {
	return te.c.LocalAddr()
}

func (te *Telnet) RemoteAddr() net.Addr {
	return te.c.RemoteAddr()
}

func (te *Telnet) SetDeadline(t time.Time) error {
	return te.c.SetDeadline(t)
}

func (te *Telnet) SetReadDeadline(t time.Time) error {
	return te.c.SetReadDeadline(t)
}

func (te *Telnet) SetWriteDeadline(t time.Time) error {
	return te.c.SetWriteDeadline(t)
}
