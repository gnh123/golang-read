## 源代码版本
```v1.17.6```

## 零、前言```io.Pipe```
```io.Pipe```可以方便多个go程, 交换[]byte, 比如有个go程不停地收音频. 你想传递给ffmpeg这个可执行文件.
所在go程. 就可以使用```io.Pipe```这个包做.

## 一、用法示例
来自pkg.go.dev
```go
package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	r, w := io.Pipe()

	go func() {
		fmt.Fprint(w, "some io.Reader stream to be read\n")
		w.Close()
	}()

	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatal(err)
	}

}

```

## 二、Write方法
```go
func (p *pipe) Write(b []byte) (n int, err error) {
        // 0.先检查下pipe是否被close, 这里使用<-p.done检查
        select {
        case <-p.done:  
                return 0, p.writeCloseError()
        default:
                p.wrMu.Lock()
                defer p.wrMu.Unlock()
        }

        // 1.先把[]byte写到wrCh chan里面
        // 2.然后检查下Read函数, 消费的具体字节数, 只有消费者(Read)消费完Write的buffer才会退出这个for循环
        // 3.如果是Write一个空的[]byte, 保证Read函数会被调用一次, 这里使用once变量
        for once := true; once || len(b) > 0; once = false {
                select {
                    
                case p.wrCh <- b: //1.见上

                        //2.见上
                        nw := <-p.rdCh
                        b = b[nw:]
                        n += nw
                case <-p.done:
                        return n, p.writeCloseError()
                } 
        }       
        return n, nil
}
```

## 三、Read方法
```go
func (p *pipe) Read(b []byte) (n int, err error) {
        // 0.先检查下pipe是否被close, 这里使用<-p.done检查
        select {
        case <-p.done:
                return 0, p.readCloseError()
        default:
        }
         
        // 1.读取buffer 
        // 2.这里比较有意思的是, 通过p.rdCh会通知Write的go程, 及通知生产者, 为什么这样做?
        //  如果接收的[]byte小于生产者送过来的buffer, 也可以正确消费完
        select {
        case bw := <-p.wrCh:// 见1
                nr := copy(b, bw)
                p.rdCh <- nr //见2
                return nr, nil
        case <-p.done:
                return 0, p.readCloseError()
        }
} 
```

## 三、通知Read结束流程(通知消费者退出)
### w.Close流程
1. 先关闭写w.Close()  
2. w.Close里面套的是CloswWithError  
3. CloseWithError套的是CloseWrite  
4. 先保存错误 
5. 然后关闭Chan, chan重复关闭会panic, 这里用once包下
### Read接口
6. ```case <-p.done:``` 这里会返回错误, 通过p.readCloseError()获取错误
```go
// 见1
func (w *PipeWriter) Close() error {
        return w.CloseWithError(nil)
}

// 见2
func (w *PipeWriter) CloseWithError(err error) error {
        return w.p.CloseWrite(err)
} 

// 见3
func (p *pipe) CloseWrite(err error) error {
        if err == nil { // 这里是关键, 标准库里面很多地方是忽略io.EOF的
                err = EOF
        }
        p.werr.Store(err) // 见4
        p.once.Do(func() { close(p.done) })// 见5
        return nil
}

func (p *pipe) readCloseError() error {
        rerr := p.rerr.Load()
        if werr := p.werr.Load(); rerr == nil && werr != nil {
                // 如果只调用Close关闭Write的pipe, 这里返回io.EOF
                return werr
        }
        return ErrClosedPipe
}
```
## 四, 难解代码解释
在Write 函数里面有加锁的超作, 很多人纳闷了, 我都用了chan, 干嘛还要加锁?  
这是因为: Write是支持分段传输[]byte的. 假如有多个go程调用Write, 不加锁内容就错了.
```go
func (p *pipe) Write(b []byte) (n int, err error) {
        select {
        case <-p.done:
                return 0, p.writeCloseError()
        default:
                p.wrMu.Lock()
                defer p.wrMu.Unlock()
        }
        // 下面的代码隐去
}
```
### 最佳实践
* 如果有多个go程传递[]byte流的需求, 可以使用io.Pipe
* 关闭生产者, 消费者可以感知到
* 关闭消费者, 生产者可以感知到
