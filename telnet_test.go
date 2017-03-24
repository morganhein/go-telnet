package go_telnet

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/jordwest/mock-conn"
	"github.com/stretchr/testify/assert"
)

func TestEscapedIAC(t *testing.T) {
	fmt.Println("")
	tel := &con{
		bIn:  bytes.NewBuffer(nil),
		bOut: bytes.NewBuffer(nil),
		quit: make(chan bool, 1),
	}

	tel.bIn.Write([]byte{IAC, IAC, 23})
	tel.processIAC()
	assert.Equal(t, []byte{IAC}, tel.bOut.Bytes())
}

func TestDo(t *testing.T) {
	tel := &con{
		bIn:  bytes.NewBuffer(nil),
		bOut: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client

	go func() {
		_, err := tel.bIn.Write([]byte{IAC, DO, ECHO})
		if err != nil {
			t.Fatal(err)
		}
		tel.processIAC()
	}()

	s := c.Server
	buf := make([]byte, 3)
	_, _ = s.Read(buf)
	assert.Equal(t, []byte{IAC, WONT, ECHO}, buf)
}

func TestWill(t *testing.T) {
	tel := &con{
		bIn:  bytes.NewBuffer(nil),
		bOut: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client

	go func() {
		_, err := tel.bIn.Write([]byte{IAC, WILL, ECHO})
		if err != nil {
			t.Fatal(err)
		}
		tel.processIAC()
		tel.c.Close()
	}()

	s := c.Server
	buf := make([]byte, 3)
	_, _ = s.Read(buf)
	assert.Equal(t, []byte{IAC, DONT, ECHO}, buf)
}

func TestWont(t *testing.T) {
	tel := &con{
		bIn:  bytes.NewBuffer(nil),
		bOut: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client

	_, err := tel.bIn.Write([]byte{IAC, WONT, ECHO})
	if err != nil {
		t.Fatal(err)
	}
	tel.processIAC()
	// todo: what to test here?
}

func TestDont(t *testing.T) {
	tel := &con{
		bIn:  bytes.NewBuffer(nil),
		bOut: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client

	go func() {
		_, err := tel.bIn.Write([]byte{IAC, DONT, ECHO})
		if err != nil {
			t.Fatal(err)
		}
		tel.processIAC()
	}()

	s := c.Server
	buf := make([]byte, 3)
	_, _ = s.Read(buf)
	assert.Equal(t, []byte{IAC, WONT, ECHO}, buf)
}

func TestBuffer(t *testing.T) {
	tel := &con{
		quit: make(chan bool, 1),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client
	s := c.Server

	go tel.buffer()

	time.Sleep(time.Duration(50) * time.Millisecond)

	go func() {
		//defer tel.c.Close()
		i, err := s.Write([]byte{IAC, DO, ECHO})
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			t.Fatal("Nothing was written to server output stream.")
		}
		time.Sleep(time.Duration(50) * time.Millisecond)
	}()

	buf := make([]byte, 3)
	i, err := s.Read(buf)
	if i != 0 {
		assert.Equal(t, []byte{IAC, WONT, ECHO}, buf)
	}

	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Duration(5) * time.Millisecond)

	tel.quit <- true
}

func TestBuffer_ForwardUpToIAC(t *testing.T) {
	tel := &con{
		quit: make(chan bool, 1),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client

	go tel.buffer()
	time.Sleep(time.Duration(50) * time.Millisecond)

	go func() {
		_, err := tel.bIn.Write([]byte{1, 2, 3, 4, 5, 6, IAC, DO, ECHO, IAC, IAC})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(50) * time.Millisecond)
	}()

	// Wait for the response
	for tel.bOut.Len() == 0 {
		time.Sleep(time.Duration(20) * time.Millisecond)
	}

	buf := make([]byte, 6)
	_, err := tel.bOut.Read(buf)

	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6}, buf)

	time.Sleep(time.Duration(50) * time.Millisecond)

	tel.quit <- true
}

func TestBuffer_ForwardUpToIACAndProcess(t *testing.T) {
	tel := &con{
		quit: make(chan bool, 1),
	}

	c := mock_conn.NewConn()
	tel.c = c.Client
	s := c.Server

	go tel.buffer()
	time.Sleep(time.Duration(50) * time.Millisecond)

	go func() {
		_, err := tel.bIn.Write([]byte{1, 2, 3, 4, 5, 6, IAC, DO, ECHO})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(50) * time.Millisecond)
	}()

	// Wait for the response
	for tel.bOut.Len() == 0 {
		time.Sleep(time.Duration(20) * time.Millisecond)
	}

	buf := make([]byte, 6)
	_, err := tel.bOut.Read(buf)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6}, buf)

	buf = make([]byte, 3)
	// Wait for the next set
	_, err = s.Read(buf)

	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []byte{IAC, WONT, ECHO}, buf)

	tel.quit <- true
	time.Sleep(time.Duration(50) * time.Millisecond)
}
