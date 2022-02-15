package main

import (
	"io"
	"runtime"
	"strings"
	"time"
)

type multiReader struct {
	readers []io.Reader
}

func (mr *multiReader) Read(p []byte) (n int, err error) {
	for len(mr.readers) > 0 {
		n, err = mr.readers[0].Read(p)
		if n > 0 || err != io.EOF {
			if err == io.EOF {
				// Don't return EOF yet. There may be more bytes
				// in the remaining readers.
				mr.readers[0] = nil
				err = nil
			}
			return
		}
		mr.readers = mr.readers[1:]
	}
	return 0, io.EOF
}

func MultiReader(readers ...io.Reader) io.Reader {
	r := make([]io.Reader, len(readers))
	copy(r, readers)
	return &multiReader{r}
}

type errorReader struct {
	err error
}

func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

func main() {
	// Read 50 bytes then produce an error
	mr1 := io.MultiReader(strings.NewReader("def"), strings.NewReader("ghi"))
	mr2 := io.MultiReader(strings.NewReader("abc"), mr1)

	io.ReadFull(mr2, make([]byte, 4))

	runtime.GC()
	time.Sleep(time.Second)

	io.ReadFull(mr1, make([]byte, 4))

	runtime.GC()
	time.Sleep(time.Second)

	io.ReadFull(mr2, make([]byte, 3))
}
