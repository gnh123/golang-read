## 源代码版本
```v1.17.6```

## 零、前言```strings.Builder```
```strings.Builder```是```go1.10```加进去的api, 在golang里面```string([]byte类型的变量)```, 底层会调用malloc分配一个新的内存, 从这里看不是太必要. 在下图中会画下```strings```和```[]byte```的内存布局. []byte的内存布局只比string多一个cap, 如果强制内存对齐也没有问题.无非是cap这个成员变量在string类型中假装不知道.
strings.Builder也是基于这个思路写的. 下面开始源代码分析

## 一、string结构体布局
```go
// strings的结构体定义, 可以在reflect找到.
// reflect.StringHeader

type StringHeader struct {
        Data uintptr
        Len  int
}
/*
┌────────────────┐
│                │
│                │
│  Data(uintptr) │
│                │
│  8byte         │
│                │
├────────────────┤
│                │
│                │
│  Len(int)      │
│                │
│  8byte         │
│                │
└────────────────┘
 */
```

## 二、[]byte布局
```go
// []byte的结构体定义, 可以在reflect找到
// reflect.SliceHeader
type SliceHeader struct {
        Data uintptr
        Len  int
        Cap  int
}

/*
┌───────────────────┐
│                   │
│   Data(uintptr)   │
│                   │
│   8byte           │
│                   │
│                   │
├───────────────────┤
│                   │
│   Len(int)        │
│                   │
│   8byte           │
│                   │
│                   │
├───────────────────┤
│                   │
│   Cap(int)        │
│                   │
│   8byte           │
│                   │
│                   │
└───────────────────┘
 */
```

### 三、Builder结构体定义
```go
// addr 用于检查Builder有没有用于指针形式的传递
// A Builder is used to efficiently build a string using Write methods.
// It minimizes memory copying. The zero value is ready to use.
// Do not copy a non-zero Builder.
type Builder struct {
        addr *Builder // of receiver, to detect copies by value
        buf  []byte
}
```

### 四、String方法
这套api的精髓, 原理也在前言部分交代了. 这样可以节省内存
```go
// String returns the accumulated string.
func (b *Builder) String() string {
        return *(*string)(unsafe.Pointer(&b.buf))
}
```

### 五、Write函数
#### 5.1 WriteString函数
```go
// 写入string类型变量
// WriteString appends the contents of s to b's buffer.
// It returns the length of s and a nil error.
func (b *Builder) WriteString(s string) (int, error) {
        b.copyCheck() //检查你是按指针传递, 还是值传递
        b.buf = append(b.buf, s...) // 追加string数据到buf里面
        return len(s), nil
}
```

#### 5.2 Write函数
```go
// 与入Write类型变量
// Write appends the contents of p to b's buffer.
// Write always returns len(p), nil.
func (b *Builder) Write(p []byte) (int, error) {
        b.copyCheck() //检查你的值按指针传递, 还是值传递
        b.buf = append(b.buf, p...) // 追加[]byte数据到buf里面
        return len(p), nil
}
```
#### 5.2 WriteByte函数
```go
// WriteByte appends the byte c to b's buffer.
// The returned error is always nil.
func (b *Builder) WriteByte(c byte) error {
        b.copyCheck() //检查你是按指针传递, 还是值传递
        b.buf = append(b.buf, c) // 追加byte类型变量到buf里面
        return nil
}
```

#### 5.3 WriteRune函数
```go
// WriteRune appends the UTF-8 encoding of Unicode code point r to b's buffer.
// It returns the length of r and a nil error.
func (b *Builder) WriteRune(r rune) (int, error) {
        b.copyCheck() // 检查你的值是按指针传递, 还是值传递
        // Compare as uint32 to correctly handle negative runes.
        if uint32(r) < utf8.RuneSelf {// 如果是ascii表里面的字符
                b.buf = append(b.buf, byte(r))
                return 1, nil
        }
        l := len(b.buf)//当前使用数据
        if cap(b.buf)-l < utf8.UTFMax {//剩余长度不够放一个最大的utf8字符
                b.grow(utf8.UTFMax) //扩容
        }
        n := utf8.EncodeRune(b.buf[l:l+utf8.UTFMax], r)//编码之后存放
        b.buf = b.buf[:l+n]
        return n, nil
}
```

### 六、Grow函数
grow函数, 用于扩容
```go
// grow copies the buffer to a new, larger buffer so that there are at least n
// bytes of capacity beyond len(b.buf).
func (b *Builder) grow(n int) {
        buf := make([]byte, len(b.buf), 2*cap(b.buf)+n)
        copy(buf, b.buf)
        b.buf = buf
}

// Grow grows b's capacity, if necessary, to guarantee space for
// another n bytes. After Grow(n), at least n bytes can be written to b
// without another allocation. If n is negative, Grow panics.
func (b *Builder) Grow(n int) {
        b.copyCheck()
        if n < 0 { 
                panic("strings.Builder.Grow: negative count")
        }
        // 剩余容量 < 需要装载的容量
        if cap(b.buf)-len(b.buf) < n { 
                b.grow(n)
        }   
}

```

### 七、精彩回顾
#### 7.1 该包最核心的点
strings.Builder库的思路建立在强制类型转换上


#### 7.2 copyCheck函数的目的--捍卫string类型的定位

copyCheck加上是为了捍卫string类型的定位, 具体可看(https://github.com/golang/go/issues/23084 )

golang里面的string代表的是不可变的字符序列, 如果不加copyCheck, 值传递的情况, 那就变成了可变. 这和string所代表的定位不一样.

可以试着去除```b.copyCheck()```, 运行下例子体会下.
```go
//This program:

package main

import (
        "fmt"
        "strings"
)

func main() {
        var b1 strings.Builder
        b1.Grow(3)
        b2 := b1
        b1.WriteString("foo")
        s := b1.String()
        fmt.Printf("string before patching: %#v\n", s)
        b2.WriteString("bar")
        fmt.Printf("string after patching: %#v\n", s)
}
//Prints:

//string before patching: "foo"
//string after patching: "bar"
```