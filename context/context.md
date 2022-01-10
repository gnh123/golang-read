## 零、前言 context
context的重要性不容置疑, 只要用过grpc或者sql的包, 都会发现这个参数. 出现频率之高在向 ``` if err != nil ``` 靠齐
本文是对context源代码的分析,全文分几个部分, 先复习下context的方法, 包级函数, 然后是如何实现, 最后一块是源代码流程账. 
结束就是如何用好context, 避免一些坑.

## 一、context 暴露的接口有
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
│               │                         │┼│                 │
│               │                         │┼│                 │
│               │                         │┼│  Err            │
│               │                         │┼│                 │
└──────┬────────┴─────────────────────────┼┼│                 │
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

 ## 二、context包级函数
* WithCancel 继承一个context, 返回cancel
* WithTimeout, 设置一个时间段之后超时
* WithDeadline, 设置截止时间
* Background, TODO 创建一个空的ctx
* WithValue, 设置值到context里面

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

 ## 三、WithValue和Value--存值和取值的流程
 从源代码上看, 每次调用WithValue就是新建一个链表的node. 每次调查是O(n)次.
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
        // 把爸爸链点包起来
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