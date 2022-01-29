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
