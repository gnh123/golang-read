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
        // 2.然后检查下Read函数, 消费的具体字节数, 只有消费完Write的buffer才会退出这个for循环
        // 3.如果是Write一个空的[]byte, 包证Read函数会被调用一次, 这里使用once变量
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
        // 2.这里比较有意思的是, 通过p.rdCh会通知写的go程, 实际的消费数, 为什么这样做: 如果接收的[]byte小于发生的buffer, 也可以正确消费完
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