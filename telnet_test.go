package gote

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jordwest/mock-conn"
	"github.com/stretchr/testify/assert"
)

func TestEscapedIAC(t *testing.T) {
	fmt.Println("")
	tel := &conn{
		i:    bytes.NewBuffer(nil),
		u:    bytes.NewBuffer(nil),
		quit: make(chan bool, 1),
	}

	tel.i.Write([]byte{IAC, IAC, 23})
	tel.processIAC()
	assert.Equal(t, []byte{IAC}, tel.u.Bytes())
}

func TestDo(t *testing.T) {
	tel := &conn{
		i: bytes.NewBuffer(nil),
		u: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.Conn = c.Client

	go func() {
		_, err := tel.i.Write([]byte{IAC, DO, ECHO})
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
	tel := &conn{
		i: bytes.NewBuffer(nil),
		u: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.Conn = c.Client

	go func() {
		_, err := tel.i.Write([]byte{IAC, WILL, ECHO})
		if err != nil {
			t.Fatal(err)
		}
		tel.processIAC()
		tel.Conn.Close()
	}()

	s := c.Server
	buf := make([]byte, 3)
	_, _ = s.Read(buf)
	assert.Equal(t, []byte{IAC, DONT, ECHO}, buf)
}

func TestWont(t *testing.T) {
	tel := &conn{
		i: bytes.NewBuffer(nil),
		u: bytes.NewBuffer(nil),
	}

	c := mock_conn.NewConn()
	tel.Conn = c.Client

	_, err := tel.i.Write([]byte{IAC, WONT, ECHO})
	if err != nil {
		t.Fatal(err)
	}
	tel.processIAC()
	// todo: what to test here?
}

func TestDont(t *testing.T) {
	tel := &conn{
		i: bytes.NewBuffer(nil),
		u: bytes.NewBuffer(nil),
		//uLock: &sync.Mutex{},
		//iLock: &sync.Mutex{},
	}

	c := mock_conn.NewConn()
	tel.Conn = c.Client

	go func() {
		_, err := tel.i.Write([]byte{IAC, DONT, ECHO})
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
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		con, err := Dial("tcp", ":3000")
		if err != nil {
			t.Fatal(err)
		}
		defer con.Close()
		time.Sleep(time.Duration(20) * time.Millisecond)
		wg.Wait()
		con.Close()
	}(&wg)

	l, err := net.Listen("tcp", ":3000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	conn, err := l.Accept()
	if err != nil {
		return
	}

	defer conn.Close()

	conn.Write([]byte{IAC, DO, ECHO})

	time.Sleep(time.Duration(20) * time.Millisecond)
	buf := bufio.NewReader(conn)
	b, err := buf.Peek(3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []byte{IAC, WONT, ECHO}, b)
	wg.Done()
}

func TestBuffer_ProcessingIAC(t *testing.T) {
	wgServer := sync.WaitGroup{}
	wgClient := sync.WaitGroup{}
	wgClient.Add(1)
	wgServer.Add(1)

	go func(wgServer *sync.WaitGroup, wgClient *sync.WaitGroup) {
		con, err := Dial("tcp", ":3000")
		if err != nil {
			t.Fatal(err)
		}
		defer con.Close()

		time.Sleep(time.Duration(20) * time.Millisecond)

		b := make([]byte, 7)
		i, err := con.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, 7, i)
		assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, IAC}, b[:i])
		wgServer.Wait()
		wgClient.Done()
	}(&wgServer, &wgClient)

	l, err := net.Listen("tcp", ":3000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	conn, err := l.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	conn.Write([]byte{1, 2, 3, 4, 5, 6, IAC, DO, ECHO, IAC, IAC})
	time.Sleep(time.Duration(20) * time.Millisecond)
	buf := bufio.NewReader(conn)
	b, err := buf.Peek(3)
	assert.NoError(t, err)
	assert.Equal(t, []byte{IAC, WONT, ECHO}, b)
	wgServer.Done()
	wgClient.Wait()
}

func TestErrorPropagation(t *testing.T) {
	wgServer := sync.WaitGroup{}
	wgClient := sync.WaitGroup{}
	wgClient.Add(1)
	wgServer.Add(1)

	go func(wgServer *sync.WaitGroup, wgClient *sync.WaitGroup) {
		con, err := Dial("tcp", ":3000")
		if err != nil {
			t.Fatal(err)
		}
		wgServer.Wait()
		b := make([]byte, 2)
		_, err = con.Read(b)
		assert.Error(t, err)
		wgClient.Done()
	}(&wgServer, &wgClient)

	l, err := net.Listen("tcp", ":3000")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	conn, err := l.Accept()
	if err != nil {
		return
	}
	conn.Close()
	wgServer.Done()
	wgClient.Wait()
}
