## 源代码版本
```v1.17.6```

## 零、前言```bytes.Buffer```
bytes.Buffer是组合数据使用频率最高的一个包之一

## 一、用法示例
```go
package main

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
)

func main() {
	// A Buffer can turn a string or a []byte into an io.Reader.
	buf := bytes.NewBufferString("R29waGVycyBydWxlIQ==")
	dec := base64.NewDecoder(base64.StdEncoding, buf)
	io.Copy(os.Stdout, dec)
}
```

## 二、定义
* buf 是底层的[]byte
* off 当前偏移量
* lastRead
``` go
// smallBufferSize is an initial allocation minimal capacity.
const smallBufferSize = 64

// A Buffer is a variable-sized buffer of bytes with Read and Write methods.
// The zero value for Buffer is an empty buffer ready to use.
type Buffer struct {
	buf      []byte // contents are the bytes buf[off : len(buf)]
	off      int    // read at &buf[off], write at &buf[len(buf)]
	lastRead readOp // last read operation, so that Unread* can work correctly.
}

```
## 三、常量
```go
// The readOp constants describe the last action performed on
// the buffer, so that UnreadRune and UnreadByte can check for
// invalid usage. opReadRuneX constants are chosen such that
// converted to int they correspond to the rune size that was read.
type readOp int8

// Don't use iota for these, as the values need to correspond with the
// names and comments, which is easier to see when being explicit.
const (
	opRead      readOp = -1 // Any other read operation.
	opInvalid   readOp = 0  // Non-read operation.
	opReadRune1 readOp = 1  // Read rune of size 1.
	opReadRune2 readOp = 2  // Read rune of size 2.
	opReadRune3 readOp = 3  // Read rune of size 3.
	opReadRune4 readOp = 4  // Read rune of size 4.
)
```

## 四、获取当前```bytes.Buffer```里面的内容
1. Bytes()返回[]byte类型
2. String()返回string类型
```go
// 见1
func (b *Buffer) Bytes() []byte { return b.buf[b.off:] }
// 见2
func (b *Buffer) String() string {
	if b == nil {
		// Special case, useful in debugging.
		return "<nil>"
	}
	return string(b.buf[b.off:])
}
```

## 五、```Len```和```Cap```
1. Len就是底层slice的长度减去off的长度得来的.
2. Cap就返回底层slice的长度
```go
// empty reports whether the unread portion of the buffer is empty.
func (b *Buffer) empty() bool { return len(b.buf) <= b.off }

// Len returns the number of bytes of the unread portion of the buffer;
// b.Len() == len(b.Bytes()).
// 见1
func (b *Buffer) Len() int { return len(b.buf) - b.off }

// Cap returns the capacity of the buffer's underlying byte slice, that is, the
// total space allocated for the buffer's data.
// 见2
func (b *Buffer) Cap() int { return cap(b.buf) }
```

## 六、```Truncate```
1. 如果传递为0, 整个bytes.Buffer的数据就会被重置
2. 截断
```go
func (b *Buffer) Truncate(n int) {
	//见1
	if n == 0 {
		b.Reset()
		return
	}
	b.lastRead = opInvalid
	if n < 0 || n > b.Len() {
		panic("bytes.Buffer: truncation out of range")
	}
	// 见2
	b.buf = b.buf[:b.off+n]
}
```

## 七、```Write```流程关键函数
### 7.0 ```makeSlice```
* 可以捕获异常的make函数
```go
func makeSlice(n int) []byte {
	// If the make fails, give a known error.
	defer func() {
		if recover() != nil {
			panic(ErrTooLarge)
		}
	}()
	return make([]byte, n)
}
```
### 7.1 ```tryGrowByReslice```函数
1. 如果剩余长度还够的话
1. 直接修改buf的Len成员变量
1. 总结: tryGrowByReslice函数就是看是你实际能否放得下, 能放得下, 就直接修改.Len放数据
```go
func (b *Buffer) tryGrowByReslice(n int) (int, bool) {
	// 见7.1 节的1
	if l := len(b.buf); n <= cap(b.buf)-l {
		// 见7.1的2
		b.buf = b.buf[:l+n]
		return l, true
	}
	return 0, false
}
```
### 7.2 ```grow```函数
1. buffer是空的情况, 重置下状态
1. 如果实际空间满足, 返回可以写的位置
1. 如果buf没有被初始化, 并且要写的值小于一个阈值, 统一都用这个值初始化
1. 如果加上将要写入的数据得到总长度, 总长度<= 实际容量的一半, 把以前的数据从右边移到左边
```go
// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with ErrTooLarge.
func (b *Buffer) grow(n int) int {
	m := b.Len()
	// 见1
	// If buffer is empty, reset to recover space.
	if m == 0 && b.off != 0 {
		b.Reset()
	}
	// 见2
	// Try to grow by means of a reslice.
	if i, ok := b.tryGrowByReslice(n); ok {
		return i
	}
	// 见3
	if b.buf == nil && n <= smallBufferSize {
		b.buf = make([]byte, n, smallBufferSize)
		return 0
	}
	// 见4
	c := cap(b.buf)
	if n <= c/2-m {
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= c to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		copy(b.buf, b.buf[b.off:])
	} else if c > maxInt-c-n {
		panic(ErrTooLarge)
	} else {
		// Not enough space anywhere, we need to allocate.
		buf := makeSlice(2*c + n)
		copy(buf, b.buf[b.off:])
		b.buf = buf
	}
	// Restore b.off and len(b.buf).
	b.off = 0
	b.buf = b.buf[:m+n]
	return m
}
```
### 7.3 ```Write```函数
```go
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(p))
	if !ok {
		m = b.grow(len(p))
	}
	return copy(b.buf[m:], p), nil
}
```