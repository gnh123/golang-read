package main

import (
	"bytes"
	"errors"
	"io"
	"log"
)

type errorReader struct {
	err error
}

func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

func main() {
	// Read 50 bytes then produce an error
	contents := "12345678901234567890123456789012345678901234567890"
	er := &errorReader{errors.New("potato")}
	in := io.MultiReader(bytes.NewBufferString(contents), er)

	// Peek a byte from the reader then reconstruct it
	buf := make([]byte, 1)
	n, err := io.ReadFull(in, buf)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Peeked %q", buf[:n])
	in = io.MultiReader(bytes.NewReader(buf[:n]), in)

	buf2 := make([]byte, 32768)
	for {
		n, err = in.Read(buf2)
		log.Printf("Read %q, err=%v, len=%d", buf2[:n], err, n)
		if err != nil {
			break
		}
	}

	// One more read
	buf3 := make([]byte, 1)
	n, err = in.Read(buf3)
	log.Printf("Read %q, err=%v, len=%d", buf3[:n], err, n)
}
