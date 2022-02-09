## 源代码版本
```v1.17.6```

## 零、前言```io.go```
io包是高频使用的包. io.Reader, io.Writer之类每天都要使用
## 一、```WriteString```接口
如果实时StringWriter就使用这个接口发送数据, 没有就转成[]byte类型再写
```go
func WriteString(w Writer, s string) (n int, err error) {
        if sw, ok := w.(StringWriter); ok {
                return sw.WriteString(s)
        }
        return w.Write([]byte(s))
}
```

## 二、```ReadAt```系列
### 2.1、```ReadAtLeast```接口
ReadAtLeast至少读取n个字节.
1. buf比min值小, 直接返回错误
2. 已读取的>= min值, 或者出错或者读到io.EOF退出读流程
3. 读的比min值还多, 忽略错误
4. 读的比最小值还少, 并且读完, 返回ErrUnexpectedEOF 错误
5. 这里有3大类情况, 1. 读取了>=min的数据, 返回nil, 2.读取一些数据少于min, 并且无数据可读(io.EOF), 返回ErrUnexpectedEOF 3.r.Read返回的其他错误
```go
func ReadAtLeast(r Reader, buf []byte, min int) (n int, err error) {
        if len(buf) < min {//见1
                return 0, ErrShortBuffer
        }
        for n < min && err == nil {//见2
                var nn int
                nn, err = r.Read(buf[n:])
                n += nn
        }
        if n >= min { //见3
                err = nil
        } else if n > 0 && err == EOF {
                err = ErrUnexpectedEOF
        }
        return //见5
}
```

### 2.2、 ```ReadFull``` 接口
ReadFull是ReadAtLeast的套壳, 读取定长数据
```go
func ReadFull(r Reader, buf []byte) (n int, err error) {
        return ReadAtLeast(r, buf, len(buf))
}
```
## 三、```io.Copy``` 系列
### 3.1 ```copyBuffer```接口
```copyBuffer```对外的接口有```io.Copy```和```io.CopyBuffer```

1. 如果src里面包含WriterTo接口, 直接调用
2. 如果dst里面包含ReaderFrom接口, 直接调用
3. 如果buffer为空, 给个32KB长度, 如果实现LimitedReader接口就取最小值(min(size, l.N))
4. 这里只有nr == nw才会继续往下写, 更新下written, 算下总共发了多少[]byte数据
5. 忽略io.EOF的读错误, 继续, 其余的错误直接返回
```go
func copyBuffer(dst Writer, src Reader, buf []byte) (written int64, err error) {
	// 见1
        if wt, ok := src.(WriterTo); ok {
                return wt.WriteTo(dst)
        }
	// 见2
        if rt, ok := dst.(ReaderFrom); ok {
                return rt.ReadFrom(src)
        }
        if buf == nil {
                size := 32 * 1024
                if l, ok := src.(*LimitedReader); ok && int64(size) > l.N {
                        if l.N < 1 {
				// 这里不写l.N < 0, 是为了避免死循环, 假如l.N为0每次读0个, 不要想走出下面的for循环
                                size = 1
                        } else { 
                                size = int(l.N)
                        } 
                }               
                buf = make([]byte, size)
        }       
        for {   
                nr, er := src.Read(buf)
                if nr > 0 {
                        nw, ew := dst.Write(buf[0:nr])
			// 见4
                        if nw < 0 || nr < nw {
                                nw = 0
                                if ew == nil {
                                        ew = errInvalidWrite
                                }
                        }               
                        written += int64(nw)
                        if ew != nil {
                                err = ew
                                break
                        }       
                        if nr != nw {
                                err = ErrShortWrite
                                break
                        }       
                }               
		// 见5
                if er != nil {
                        if er != EOF {
                                err = er
                        }
                        break   
                }       
        }               
        return written, err
}       

```

##  四 ```io.LimitedReader``` 限制读数据
1. LimitReader是构造函数, 对一个io.Reader最多读多少数据
2. LimitedReader是结构体的原型
3. 如果被读数据```len(p)```大于最多读取```l.N```的量, 则只读l.N的量
4. 读数据, 递减```l.N```的值

```go
// 见1
// LimitReader returns a Reader that reads from r
// but stops with EOF after n bytes.
// The underlying implementation is a *LimitedReader.
func LimitReader(r Reader, n int64) Reader { return &LimitedReader{r, n} }

// A LimitedReader reads from R but limits the amount of
// data returned to just N bytes. Each call to Read
// updates N to reflect the new amount remaining.
// Read returns EOF when N <= 0 or when the underlying R returns EOF.
type LimitedReader struct {
        R Reader // underlying reader
        N int64  // max bytes remaining
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	// 读结束
        if l.N <= 0 {
                return 0, EOF
        }
	// 见3
        if int64(len(p)) > l.N {
                p = p[0:l.N]
        }
	// 见4
        n, err = l.R.Read(p)
        l.N -= int64(n)
        return
}
```
## 五、```io.SectionReader```
1. ```s.off >= s.limit``` 说明数据都读完了
2. 读min(len(p), s.limit)长度的数据
3. 读数据, 并且更新```s.off```
```go
// 初始化函数
func NewSectionReader(r ReaderAt, off int64, n int64) *SectionReader {
    return &SectionReader{r, off, off, off + n}
}

// SectionReader implements Read, Seek, and ReadAt on a section
// of an underlying ReaderAt.
type SectionReader struct {
    r     ReaderAt //数据源
    base  int64    //起始地址
    off   int64    //当前位置
    limit int64    //最多读多少数据
}

func (s *SectionReader) Read(p []byte) (n int, err error) {
    // 见1
    if s.off >= s.limit {
        return 0, EOF 
    } 
    // 见2
    if max := s.limit - s.off; int64(len(p)) > max {
        p = p[0:max]
    }
    // 见3 
    n, err = s.r.ReadAt(p, s.off)
    s.off += int64(n)
    return
}
```
1. 设置SeekStart时, 就把offset转成从base处计算的绝对值, SeekStart++++offset
2. 设置SeekCurrent时, 就把offset转成从s.off当前处计算的绝对值, SeekStart++++++offset
3. 设置SeekEnd时, 就把offset转成从s.limit(结尾处)计算的绝对值, 一般会使用负值, SeekEnd+++++offset
```go
var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

func (s *SectionReader) Seek(offset int64, whence int) (int64, error) {
    switch whence {
    default:
        // 异常值判断
        return 0, errWhence
    case SeekStart:
        //见1
        offset += s.base
    case SeekCurrent:
        //见2
        offset += s.off
    case SeekEnd:
        //见3
        offset += s.limit
    }
    // 异常值判断
    if offset < s.base {
        return 0, errOffset
    }
    // 重置s.off
    s.off = offset
    // 返回偏移的长度
    return offset - s.base, nil
}

func (s *SectionReader) ReadAt(p []byte, off int64) (n int, err error) {
        //off小于0, 或者off超过剩下的距离, 直接认为结束
    if off < 0 || off >= s.limit-s.base {
        return 0, EOF
    }
    off += s.base//把相对值, 转对绝对值
    if max := s.limit - off; int64(len(p)) > max {
        p = p[0:max]//取最小可读长度
        n, err = s.r.ReadAt(p, off)
        if err == nil {
            err = EOF
        }
        return n, err
    }
    return s.r.ReadAt(p, off)
}

// Size returns the size of the section in bytes.
func (s *SectionReader) Size() int64 { return s.limit - s.base }
```

## 六、```TeeReader```
TeeReader函数有点意思, 会旁路一个io.Writer出去. 当io.Reader被调用时, io.Writer得在得到数据
```go
func TeeReader(r Reader, w Writer) Reader {
    return &teeReader{r, w}
}

type teeReader struct {
    r Reader
    w Writer
}

func (t *teeReader) Read(p []byte) (n int, err error) {
    n, err = t.r.Read(p)
    if n > 0 { 
        if n, err := t.w.Write(p[:n]); err != nil {
            return n, err 
        }   
    }   
    return
}

```

## 七、```Discard``` 黑洞
要丢弃一些数据的时候使用

```go
// Discard is a Writer on which all Write calls succeed
// without doing anything.
var Discard Writer = discard{}

type discard struct{}

// discard implements ReaderFrom as an optimization so Copy to
// io.Discard can avoid doing unnecessary work.
var _ ReaderFrom = discard{}

func (discard) Write(p []byte) (int, error) {
    return len(p), nil
}

func (discard) WriteString(s string) (int, error) {
    return len(s), nil
}
```