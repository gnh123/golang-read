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
```
// String returns the accumulated string.
func (b *Builder) String() string {
        return *(*string)(unsafe.Pointer(&b.buf))
}
```