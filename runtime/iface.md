## 零、iface文件分析
go1.18 版本编译器
## 一、分析把int64类型的变量放到interface{}的过程
### 1.1 实验代码
```go tool compile -l -S -N  set_int_to_interface.go``` 其中```-N```可以关掉编译优化选项
```go
package main

var i interface{}

func main() {
	var y = 888
	i = y

    z := i.(int)
	println(i, z)
}
```
```go
	0x0018 00024 (set_int_to_interface.go:6)	MOVQ	$888, "".y+32(SP)
	0x0021 00033 (set_int_to_interface.go:7)	MOVL	$888, AX//没看明白, 不知为啥又放到寄存器中
	0x0026 00038 (set_int_to_interface.go:7)	PCDATA	$1, $0
	0x0026 00038 (set_int_to_interface.go:7)	CALL	runtime.convT64(SB) // 调用runtime.convT64函数
	0x002b 00043 (set_int_to_interface.go:7)	MOVQ	AX, ""..autotmp_3+48(SP)
	0x0030 00048 (set_int_to_interface.go:7)	LEAQ	type.int(SB), CX //指针变量拷贝到eface._type里面
	0x0037 00055 (set_int_to_interface.go:7)	MOVQ	CX, "".i(SB)
	0x003e 00062 (set_int_to_interface.go:7)	PCDATA	$0, $-2
```
### 1.2 eface 内存结构
eface是```interface{}```在runtime的底层结构
```go
// 源代码位于 ./src/runtime/iface.go

type eface struct {
    _type *_type
    data  unsafe.Pointer
}
```

### 1.2 ```conveT64``` 源代码
conveT64, 传入888就malloc一块空间给888存起来, 这块空间的指针最后给到eface.data成员变量存储起来. (convT64在这里有个优化, 如果val 小于256, 直接staticuint64s表里面的值, 免于一次malloc
```go
// 源代码位于
func convT64(val uint64) (x unsafe.Pointer) {
	if val < uint64(len(staticuint64s)) {
		x = unsafe.Pointer(&staticuint64s[val])
	} else {
		x = mallocgc(8, uint64Type, false)
		*(*uint64)(x) = val
	}
	return
}
```
