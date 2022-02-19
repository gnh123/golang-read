## 源代码版本
```v1.17.6```

## 零、前言```bytes.Buffer```
bytes.Buffer是组合数据使用频率最高的一类包

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
1. 如果加上将要写入的数据得到总长度, 总长度<= 实际容量的一半, 把以前的数据从右边移到
左边
1. 如果实际的长度的2倍+需要写的长度  > 最长值. 直接panic, 如需扩容也是 2 * c + n的扩, 所以扩容之前先判断下. 这是重点操作
1. 扩容数据 2 * c + n, 移动数据, 重置off变量
1. 总结:grow就是分配空间的函数. 至少会腾出n的空间放数据, 返回可以写的位置
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
		// 见5
		// We can slide things down instead of allocating a new
		// slice. We only need m+n <= c to slide, but
		// we instead let capacity get twice as large so we
		// don't spend all our time copying.
		copy(b.buf, b.buf[b.off:])
	} else if c > maxInt-c-n {
		panic(ErrTooLarge)
	} else {
		// 见6
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
1. 实际空间够的话, 直接修改下slice的len指针
1. 实际空间不够进入grow函数
1. 拷贝数据, 完事
```go
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	// 见1
	m, ok := b.tryGrowByReslice(len(p))
	if !ok {
		// 见2
		m = b.grow(len(p))
	}
	// 见3
	return copy(b.buf[m:], p), nil
}
```
### 7.4 ```WriteString```函数
Write和WriteString一样, 没啥可说的
```go
func (b *Buffer) WriteString(s string) (n int, err error) {
	b.lastRead = opInvalid
	m, ok := b.tryGrowByReslice(len(s))
	if !ok {
		m = b.grow(len(s))
	}
	return copy(b.buf[m:], s), nil
}
```

### 7.5 ```WriteByte```函数
```go
// WriteByte appends the byte c to the buffer, growing the buffer as needed.
// The returned error is always nil, but is included to match bufio.Writer's
// WriteByte. If the buffer becomes too large, WriteByte will panic with
// ErrTooLarge.
func (b *Buffer) WriteByte(c byte) error {
	b.lastRead = opInvalid
	// 尝试分配1字节, 如果cap-1-实际写入的容量供的话
	m, ok := b.tryGrowByReslice(1)
	if !ok {
		// 实际分配
		m = b.grow(1)
	}
	// 赋值
	b.buf[m] = c
	return nil
}

```

### 7.6 ```WriteRune```函数
```go
1. 如果r是个ascii码字符, 直接调用WriteByte写入
2. 尝试分配
3. 实际分配
4. 把rune编码到[]byte里面
// WriteRune appends the UTF-8 encoding of Unicode code point r to the
// buffer, returning its length and an error, which is always nil but is
// included to match bufio.Writer's WriteRune. The buffer is grown as needed;
// if it becomes too large, WriteRune will panic with ErrTooLarge.
func (b *Buffer) WriteRune(r rune) (n int, err error) {
	// Compare as uint32 to correctly handle negative runes.
	// 见1
	if uint32(r) < utf8.RuneSelf {
		b.WriteByte(byte(r))
		return 1, nil
	}
	b.lastRead = opInvalid
	// 见2
	m, ok := b.tryGrowByReslice(utf8.UTFMax)
	if !ok {
		// 见3
		m = b.grow(utf8.UTFMax)
	}
	// 见4
	n = utf8.EncodeRune(b.buf[m:m+utf8.UTFMax], r)
	b.buf = b.buf[:m+n]
	return n, nil
}
```

### 7.7 ```ReadFrom```函数
1. 先预分配512字节的空间, 此版本的minRead= 512, grow函数腾讯会腾出512字节的空间(可能实际不止)
2. 修改[]byte结构体的Len成员变量
3. 根据grow函数返回可以写的位置, ```r.Read(i:cap(b.buf))```直接填完整个buf
4. 修改[]byte结构体的Len成员变量
5. 统计已读数据

```go
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	b.lastRead = opInvalid
	for {
		// 见1
		i := b.grow(MinRead)
		// 见2
		b.buf = b.buf[:i]
		// 见3
		m, e := r.Read(b.buf[i:cap(b.buf)])
		if m < 0 {
			panic(errNegativeRead)
		}

		// 见4
		b.buf = b.buf[:i+m]
		// 见5
		n += int64(m)
		// 读结束
		if e == io.EOF {
			return n, nil // e is EOF, so return nil explicitly
		}
		// 错误返回
		if e != nil {
			return n, e
		}
	}
}
```
## 八、```Read```关键流程函数
### 8.1 ```WriteTo```函数
1. 此时buf内部有数据
2. 直接把[]byte里面的数据写走
3. 更新off, 标识下还可以写的数据
4. buf都写走了, 重置下数据
```go
// WriteTo writes data to w until the buffer is drained or an error occurs.
// The return value n is the number of bytes written; it always fits into an
// int, but it is int64 to match the io.WriterTo interface. Any error
// encountered during the write is also returned.
func (b *Buffer) WriteTo(w io.Writer) (n int64, err error) {
	b.lastRead = opInvalid
	// 见1
	if nBytes := b.Len(); nBytes > 0 {
		// 见2
		m, e := w.Write(b.buf[b.off:])
		// 异常判断
		if m > nBytes {
			panic("bytes.Buffer.WriteTo: invalid Write count")
		}
		// 见3
		b.off += m
		// 已写走的数据
		n = int64(m)
		if e != nil {
			return n, e
		}
		// 异常判断
		// all bytes should have been written, by definition of
		// Write method in io.Writer
		if m != nBytes {
			return n, io.ErrShortWrite
		}
	}
	// Buffer is now empty; reset.
	b.Reset()
	return n, nil
}
```

### 8.2 ```Read```函数
1. 如果buf里面的数据都被取完. 或者就没写过数据, 先Reset()状态
2. 盛数据的p是空的, 直接返回. p不为空, 就返回io.EOF
3. copy数据走, 更新off
4. 如果取的数据> 0, 设置下lastRead的flag是opRead
```go
// Read reads the next len(p) bytes from the buffer or until the buffer
// is drained. The return value n is the number of bytes read. If the
// buffer has no data to return, err is io.EOF (unless len(p) is zero);
// otherwise it is nil.
func (b *Buffer) Read(p []byte) (n int, err error) {
	b.lastRead = opInvalid
	// 见1
	if b.empty() {
		// 见1
		// Buffer is empty, reset to recover space.
		b.Reset()
		// 见2
		if len(p) == 0 {
			return 0, nil
		}
		// 见2
		return 0, io.EOF
	}
	// 见3
	n = copy(p, b.buf[b.off:])
	// 见3
	b.off += n
	// 见4
	if n > 0 {
		b.lastRead = opRead
	}
	return n, nil
}
```

### 8.3 ```Next```函数
1. 先重置下flag
2. 取最小长度
3. 返回这段长度的数据(浅引用), 并且更新off值, 更新lastRead为opRead
4. 总结: 和Read做的事情有点相似, 一个是copy数据, 一个是返回的引用
```go
// Next returns a slice containing the next n bytes from the buffer,
// advancing the buffer as if the bytes had been returned by Read.
// If there are fewer than n bytes in the buffer, Next returns the entire buffer.
// The slice is only valid until the next call to a read or write method.
func (b *Buffer) Next(n int) []byte {
	// 见1
	b.lastRead = opInvalid
	// 见2
	m := b.Len()
	// 见2
	if n > m {
		// 见2
		n = m
	}
	// 见3
	data := b.buf[b.off : b.off+n]
	// 见3
	b.off += n
	if n > 0 {
		b.lastRead = opRead
	}
	return data
}
```

### 8.4 ```ReadByte```函数
1. 如果buf里面的数据都被取完. 或者就没写过数据, 先Reset()状态, 直接返回io.EOF
2. copy数据走, 更新off
3. 设置下lastRead的flag是opRead
```go
// ReadByte reads and returns the next byte from the buffer.
// If no byte is available, it returns error io.EOF.
func (b *Buffer) ReadByte() (byte, error) {
	// 见1
	if b.empty() {
		// 见1
		// Buffer is empty, reset to recover space.
		b.Reset()
		return 0, io.EOF
	}
	// 见2
	c := b.buf[b.off]
	// 见2
	b.off++
	// 见3
	b.lastRead = opRead
	return c, nil
}
```

### 8.5 ```ReadRune```函数
1. 如果buf里面的数据都被取完. 或者就没写过数据, 先Reset()状态, 直接返回io.EOF
2. 取ascii码的逻辑;先取一个字节, 恩是ascii码, 直接走起. 设置lastRead是opReadRune1标识
3. 取utf8码的逻辑; 先解码得到unicode和长度. 最后设置lastRead就走
```go
// ReadRune reads and returns the next UTF-8-encoded
// Unicode code point from the buffer.
// If no bytes are available, the error returned is io.EOF.
// If the bytes are an erroneous UTF-8 encoding, it
// consumes one byte and returns U+FFFD, 1.
func (b *Buffer) ReadRune() (r rune, size int, err error) {
	// 见1
	if b.empty() {
		// 见1
		// Buffer is empty, reset to recover space.
		b.Reset()
		// 见1
		return 0, 0, io.EOF
	}
	// 见2
	c := b.buf[b.off]
	// 见2
	if c < utf8.RuneSelf {
		// 见2
		b.off++
		// 见2
		b.lastRead = opReadRune1
		return rune(c), 1, nil
	}
	// 见3
	r, n := utf8.DecodeRune(b.buf[b.off:])
	// 见3
	b.off += n
	// 见3
	b.lastRead = readOp(n)
	return r, n, nil
}
```

### 8.6 ```UnreadRune```函数
1. 异常检测
2. off如果大于b.lastRead, 回退一个b.lastRead的长度
3. 总结:回退一个Rune
```go
// UnreadRune unreads the last rune returned by ReadRune.
// If the most recent read or write operation on the buffer was
// not a successful ReadRune, UnreadRune returns an error.  (In this regard
// it is stricter than UnreadByte, which will unread the last byte
// from any read operation.)
func (b *Buffer) UnreadRune() error {
	// 见1
	if b.lastRead <= opInvalid {
		return errors.New("bytes.Buffer: UnreadRune: previous operation was not a successful ReadRune")
	}
	// 见2
	if b.off >= int(b.lastRead) {
		b.off -= int(b.lastRead)
	}
	b.lastRead = opInvalid
	return nil
}

```

### 8.7 ```UnreadByte```函数
1. 异常检测
2. 重置lastRead为opInvalid
3. off的值--, 回退一个字节的长度
```go
func (b *Buffer) UnreadByte() error {
	// 见1
	if b.lastRead == opInvalid {
		return errUnreadByte
	}
	// 见2
	b.lastRead = opInvalid
	if b.off > 0 {
		b.off--
	}
	return nil
}
```

### 8.8 ```ReadBytes```函数
1. 读取一个slice(注意是浅引用)
2. 重新返回一个深拷贝
```go
func (b *Buffer) ReadBytes(delim byte) (line []byte, err error) {
	// 见1
	slice, err := b.readSlice(delim)
	// return a copy of slice. The buffer's backing array may
	// be overwritten by later calls.
	// 见2
	line = append(line, slice...)
	return line, err
}
```

### 8.9 ```readSlice```函数
1. 使用IndexByte查找字符, 算出结束的位置. 如果IndexByte返回<0, 则说明byte已经io.EOF
2. 取出off至end之间的值. 返回时使用
3. 更新b.off和lastRead的flag

```go
// readSlice is like ReadBytes but returns a reference to internal buffer data.
func (b *Buffer) readSlice(delim byte) (line []byte, err error) {
	// 见1
	i := IndexByte(b.buf[b.off:], delim)
	// 见1
	end := b.off + i + 1
	if i < 0 {
		end = len(b.buf)
		err = io.EOF
	}
	// 见2
	line = b.buf[b.off:end]
	// 见3
	b.off = end
	// 见3
	b.lastRead = opRead
	return line, err
}

```
### 8.10 ```ReadString```函数
1. 读取一个slice(注意是浅引用)
2. 重新返回一个深拷贝, string会malloc一块新内存
```go
func (b *Buffer) ReadString(delim byte) (line string, err error) {
	// 见1
	slice, err := b.readSlice(delim)
	// 见2
	return string(slice), err
}
```

## 九、可能的疑问
### 9.1 read/write成对使用, 内存会爆炸吗?
如果bytes.Buffer, 一端不停地写, 一端不停地读, 内存会不会炸裂  
不会.
在Write流程的grow函数里面会这样一行代码, 会把后面的数据移动到前面.
```
copy(b.buf, b.buf[b.off:])
```

