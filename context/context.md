## 源代码版本
```v1.17.6```

## 零、前言 context

context的重要性不容置疑, 只要用过grpc或者sql的包, 都会发现这个参数. 出现频率之高在向 ``` if err != nil ``` 靠齐, 
本文是对context源代码的分析,全文分几个部分, 先复习下context的方法, 包级函数, 然后是如何实现, 最后一块是源代码流程账. 
结束就是如何用好context, 避免一些坑.

## 一、context 暴露的接口有
```go
/*
                                          ┌───────────────────┐
                                          │                   │
┌───────────────┬─────────────────────────┤     Deadline      │
│               │                         │                   │
│               │                         │                   │
│               │                         └───────────────────┘
│               │
│               │
│               │                         ┌───────────────────┐
│   context     │                         │                   │
│               ├─────────────────────────┤      Done         │
│               │                         │                   │
│               │                         │                   │
│               │                         └───────────────────┘
│               │
│               │
│               │                         ┌─┬─────────────────┐
│               │                         │                   │
│               │                         │                   │
│               │                         │    Err            │
│               │                         │                   │
└──────┬────────┴─────────────────────────┼                   │
       │                                  └─┴─────────────────┘
       │
       │
       │
       │                                  ┌───────────────────┐
       │                                  │                   │
       │                                  │                   │
       │                                  │    Value          │
       └──────────────────────────────────►                   │
                                          │                   │
                                          │                   │
                                          └───────────────────┘
 */
```
 ## 二、context包级函数
* WithCancel 继承一个context, 返回新的ctx, 和cancel
* WithTimeout, 设置一个时间段之后超时
* WithDeadline, 设置截止时间
* Background, TODO 创建一个空的ctx
* WithValue, 设置值到context里面
```go
 /*
                                ┌─────────────┐
                                │             │
                                │ WithCancel  │
           ┌────────────────────┤             │
           │                    │             │
           │                    └─────────────┘
           │
           │
           │                    ┌─────────────┐
           │                    │             │
           │ ┌──────────────────┤ WithDeadline│
           │ │                  │             │
           │ │                  │             │
           │ │                  └─────────────┘
           │ │
           │ │
           │ │                  ┌─────────────┐
┌──────────┴─┴┐                 │             │
│             ├─────────────────┤ WithTimeout │
│ func        │                 │             │
│             │                 │             │
│             │                 └─────────────┘
└─────┬────┬─┬┘
      │    │ │
      │    │ │
      │    │ │                  ┌─────────────┐
      │    │ │                  │             │
      │    │ │                  │ Background  │
      │    │ └──────────────────┤             │
      │    │                    │             │
      │    │                    └─────────────┘
      │    │
      │    │
      │    │
      │    │                    ┌─────────────┐
      │    │                    │             │
      │    └────────────────────┤ TODO        │
      │                         │             │
      │                         │             │
      │                         └─────────────┘
      │
      │
      │
      │                         ┌─────────────┐
      │                         │             │
      │                         │ WithValue   │
      └─────────────────────────┤             │
                                │             │
                                └─────────────┘
 */
```
 ## 三、WithValue和Value--存值和取值的流程
 从源代码上看, 每次调用```WithValue(ctx, key, val)```就是新建一个链表的node. 每次通过```ctx.Value(key名)```函数查找key, 效率是O(n)次.
 ```go
 /*
                                                 ┌───────┐
                                                 │       │
                                                 │parent │
                                                 │       │
                                                 └──▲────┘
                                                    │
                                                    │
                        ┌───────┐                   │
                        │       │                   │
                        │ child ├───────────────────┘
                        │       │
                        └───▲───┘
                            │
                            │
                            │
┌─────────┐                 │
│         ├─────────────────┘
│grandson │
│         │
└─────────┘
 */
 ```
### 3.1 核心代码
* 链表加节点 ```&valueCtx{parent, key, val}```
* 查找链表中的元素 ```c.Context.Value(key) ```

 ```go
 func WithValue(parent Context, key, val interface{}) Context {
        if parent == nil {
                panic("cannot create context from nil parent")
        }   
        if key == nil {
                panic("nil key")
        }   
        if !reflectlite.TypeOf(key).Comparable() {
                panic("key is not comparable")
        }   
        // 把爸爸节点包起来, 使用Context接口指向
        // 这里和常规的写法不一样, 一般是Next常量, 还是那个姿势
        return &valueCtx{parent, key, val}
}

// A valueCtx carries a key-value pair. It implements Value for that key and
// delegates all other calls to the embedded Context.
type valueCtx struct {
        Context
        key, val interface{}
}

// stringify tries a bit to stringify v, without using fmt, since we don't
// want context depending on the unicode tables. This is only used by
// *valueCtx.String().
func stringify(v interface{}) string {
        switch s := v.(type) {
        case stringer:
                return s.String()
        case string:
                return s
        }   
        return "<not Stringer>"
}

func (c *valueCtx) String() string {
        return contextName(c.Context) + ".WithValue(type " +
                reflectlite.TypeOf(c.key).String() +
                ", val " + stringify(c.val) + ")" 
}

func (c *valueCtx) Value(key interface{}) interface{} {
         // 如果就是要找的key, 直接返回
        if c.key == key {
                return c.val
        }   
        // 调用它的父辈节点. 不停访问链表的下个节点
        return c.Context.Value(key)
}

 ```

 ## context 父ctx影响子ctx的做法
 ### 核心代码
 每个ctx都有一个c.dhildren. 记录了它的所有子ctx. 只要父cancel了. 会尝试优先遍历把孩子都cancel掉

 ```go
 c.children
 ```

 ```go
 /*
                            ┌──────┐
                            │      │
    ┌──────────────┬────────┤parent├─────────────┬────────────┐
    │              ├────────┴─────┬┴─────────────┤            │
    │              │              │              │            │
    │              │              │              │            │
    │              │              │              │            │
    │              │              │              │            │
    │              │              │              │            │
    │              │              │              │            │
    │              │              │              │            │
┌───▼───┐      ┌───▼───┐      ┌───▼────┐   ┌─────▼──┐   ┌─────▼──┐
│       │      │       │      │        │   │        │   │        │
│child  │      │child  │      │child   │   │ child  │   │child   │
└───────┘      └───────┘      └────────┘   └────────┘   └────────┘
 */
 ```

 ```go
 // cancel closes c.done, cancels each of c's children, and, if
// removeFromParent is true, removes c from its parent's children.
func (c *cancelCtx) cancel(removeFromParent bool, err error) {
        if err == nil {
                panic("context: internal error: missing cancel error")
        }
        c.mu.Lock()
        if c.err != nil {
                c.mu.Unlock()
                return // already canceled
        }
        c.err = err
        // TODO, 理下为啥要这么做
        d, _ := c.done.Load().(chan struct{})
        if d == nil {          
                c.done.Store(closedchan)
        } else {               
                close(d)       
        }
        // 这里递归遍历
        for child := range c.children {
                // NOTE: acquiring the child's lock while holding parent's lock.
                child.cancel(false, err)
        }
        c.children = nil
        c.mu.Unlock()

        if removeFromParent {
                removeChild(c.Context, c)
        }
}

 ```