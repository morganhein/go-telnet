package gote_test

import "github.com/morganhein/go-telnet"

func ExampleDial() {
	// Dial the telnet server
	conn, err := gote.Dial("tcp", "rainmaker.wunderground.com")
	if err != nil {
		panic("Unable to connect.")
	}
	// Read 30 bytes from the stream
	buf := make([]byte, 30)
	_, err = conn.Read(buf)
	if err != nil {
		panic("Unable to read from stream.")
	}

	// Write 'Hello World' to the stream.
	_, err = conn.Write([]byte("Hello world!"))
}
