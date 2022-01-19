## 源代码版本
```v1.17.6```

## 零、前言 errors
以前的错误返回判断是比较麻烦的. 特别是A错误包含B错误.处理这种情况比较实用不推荐的方法是字符串搜索. 
细想下, 还是挺容易出错, 比如C错误包含not found, B里面也有not found. 这里就容易出错. 幸好, 现在的
标准库, 加入结构化错误的方法.

## 一、错误装包
### 1.0 原理
err装包, 也是一个把err装载到链表中的过程, 所以现在的error不仅是错误, 也可以是一个错误链.
### 1.1 用法
```go
e  := errors.New("first")
e2 := fmt.Errorf("second: %w", e)
```
### 1.2 源代码
```go
/*
                      ┌───────────────────────┐
                      │error                  │
                      │     ┌──────┬──────┐   │
                      │     │msg   │ err  │   │
                      │     │      │      │   │
                      │     └──────┴──────┘   │
                      │                       │
                      │                       │
                      │                       │
                      └─────────────▲─────────┘
                                    │
                                    │
                                    │
                                    │
                                    │
┌───────────────────────────┬───────┘
│error                      │
│                           │
│                           │
│       ┌──────┬──────┐     │
│       │msg   │err   │     │
│       │      │      │     │
│       └──────┴──────┘     │
│                           │
│                           │
│                           │
│                           │
└───────────────────────────┘
 */
```
```go
func Errorf(format string, a ...interface{}) error {
        p := newPrinter()
        p.wrapErrs = true
        p.doPrintf(format, a)
        s := string(p.buf)
        var err error
        if p.wrappedErr == nil {
                err = errors.New(s)
        } else {
                // 使用实现Error和Unwrap的接口类型包装下
                err = &wrapError{s, p.wrappedErr}
        }
        p.free()
        return err
}

type wrapError struct {
        msg string
        err error
}       
        
func (e *wrapError) Error() string {
        return e.msg
}       

// 错误解包的函数
func (e *wrapError) Unwrap() error {
        return e.err
}
```

## 二、错误解包
由于e2已经是一个错误链表, 那调用一次```errors.Unwrap``` 就是从表头取一个元素出现
```go
e  := errors.New("first")
e2 := fmt.Errorf("second: %w", e)
fmt.Println(errors.Unwrap(e2) == e)
```

## 三、```Is```接口
遍历错误链表, 判断你的错误是否在这个链中, 遇到第一个匹配成功的就返回
这里还是看下图1. Is 就是for循环遍历链表找err
```go
func Is(err, target error) bool {
        if target == nil {
                return err == target
        }
        
        // 判断这个类型是否能比较
        isComparable := reflectlite.TypeOf(target).Comparable()
        for {   

                // 能比较, 并且相等, 说明错误链中有这个错误
                if isComparable && err == target {
                        return true
                }
                // 这个错误实现了自己的Is方法. 直接用它的Is方法
                if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
                        return true
                }
                // TODO: consider supporting target.Is(err). This would allow
                // user-definable predicates, but also may allow for coping with sloppy
                // APIs, thereby making it easier to get away with them.

                // 老老实实Unwrap, 一层一层的剥开错误类型
                if err = Unwrap(err); err == nil {
                        return false
                }
        }
}
```

## 四、```As```接口
```go
func As(err error, target interface{}) bool {
        // target空值panic
        if target == nil {
                panic("errors: target cannot be nil")
        }   
        // 先反射
        val := reflectlite.ValueOf(target)
        // 拿到类型
        typ := val.Type()
        // 不是指针或者是有类型但是是nil指针报错
        // 比如(*x)(nil)这种写法就会进val.IsNil(), 请看当前目录下的isnil.go文件, 运行下
        if typ.Kind() != reflectlite.Ptr || val.IsNil() {
                panic("errors: target must be a non-nil pointer")
        }
        // 再判断解引用之后的类型    
        targetType := typ.Elem()
        if targetType.Kind() != reflectlite.Interface && !targetType.Implements(errorType) {
                panic("errors: *target must be interface or implement error")
        }
        for err != nil {
		// 检查下是这个类型实现的error接口
                if reflectlite.TypeOf(err).AssignableTo(targetType) {
                        val.Elem().Set(reflectlite.ValueOf(err))
                        return true
                }

                // 如果传递过来的接口, 自己实现了As接口, 直接使用As接口
                if x, ok := err.(interface{ As(interface{}) bool }); ok && x.As(target) {
                        return true
                }
                // 解包
                err = Unwrap(err)
        }
        return false
}
```

## 最佳实践
* err的包装层次不易过多, 每包装一次, 他是在错误链表中不停加节点