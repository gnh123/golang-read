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

 ## context 父ctx影响子ctx的做法--cancel函数
 ### 核心代码
 每个ctx都有一个c.children, 类型是map. 记录了它的所有子ctx. 只要父cancel了. 会深度优先遍历把孩子都cancel掉.
 从这个角度看, 像一棵树. 当然parent和child 如果是一脉单传, 那它又是一个链表

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
        // TODO(check), 如果不调用Done()方法, 直接cancel会进这个流程
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
## parentCancelCtx 函数
```go
// &cancelCtxKey is the key that a cancelCtx returns itself for.
var cancelCtxKey int 

// parentCancelCtx returns the underlying *cancelCtx for parent.
// It does this by looking up parent.Value(&cancelCtxKey) to find
// the innermost enclosing *cancelCtx and then checking whether
// parent.Done() matches that *cancelCtx. (If not, the *cancelCtx
// has been wrapped in a custom implementation providing a
// different done channel, in which case we should not bypass it.)
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
        done := parent.Done()
        if done == closedchan || done == nil {
                return nil, false
        }   

	// 超级牛逼的亮点, 以前的context是不建议内嵌的. 
	// 这种写法, 内嵌官方的context也没问题
        p, ok := parent.Value(&cancelCtxKey).(*cancelCtx)
        if !ok {
                return nil, false
        }   
        pdone, _ := p.done.Load().(chan struct{})
        if pdone != done {
                return nil, false
        }   
        return p, true
}

```
## context-propagateCancel 函数
```go
func propagateCancel(parent Context, child canceler) {
        // 这里的parent只有context.TODO()或者
        // context.Background() 创建出来的会返回nil
        // 自定义context的除外
        done := parent.Done()
        if done == nil {
                return // parent is never canceled
        }
        
        // 发现parent被销毁了
        // 直接把儿子也销毁, 这里的递归调用
        select {
        case <-done:
                // parent is already canceled
                child.cancel(false, parent.Err())
                return 
        default:        
        }                       
        
        // 从接口里面取到父context的具体实现类型
        if p, ok := parentCancelCtx(parent); ok {
                p.mu.Lock()
                if p.err != nil {
                        // 发现被cancel, 把后代都cancel了
                        // parent has already been canceled
                        child.cancel(false, p.err)
                } else {
                        // 如果p.children是空
                        // 给map赋个值, 惰性初始化的写法, 让分配推迟到发生的那一该再分配内存
                        if p.children == nil {
                                p.children = make(map[canceler]struct{})
                        }       
                        p.children[child] = struct{}{}
                }       
                p.mu.Unlock()
        } else {
                // 自定义实现的context或者parent已经被cancel的进这里
                atomic.AddInt32(&goroutines, +1)
                go func() {
                        select {
                        case <-parent.Done():
                                child.cancel(false, parent.Err())
                        case <-child.Done():
                        }
                }()
        }
}

```

### 自定义context会进else部分
证实下else的部分逻辑
```go
type myContext struct {
}

func (c *myContext) Deadline() (deadline time.Time, ok bool) { 
        return
}

func (c *myContext) Done() <-chan struct{} {
        return make(chan struct{})      
}

func (c *myContext) Err() error {
        return nil
}

func (*myContext) Value(key interface{}) interface{} {
        return nil
}

func main() {
        ctx := context.TODO()

        _, cancel := context.WithCancel(ctx)
        defer cancel()

        _, cancel = context.WithCancel(&myContext{})
        defer cancel()
}
```
## context-WithDeadline函数解析
```go
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
        if parent == nil {
                panic("cannot create context from nil parent")
        }   
        // 如果父ctx超时时间早于子ctx.
        // 由于ctx的cancel是自上而下地取消的. 所以子ctx返回一个普通的带的cancel的就行
        if cur, ok := parent.Deadline(); ok && cur.Before(d) {
                // The current deadline is already sooner than the new one.
                return WithCancel(parent)
        }
        // 构建timerCtx结构   
        c := &timerCtx{
                cancelCtx: newCancelCtx(parent),
                deadline:  d,  
        } 
          
        propagateCancel(parent, c)
        // 如果时间已到, 直接cancel. 然后返回
        dur := time.Until(d)
        if dur <= 0 { 
                c.cancel(true, DeadlineExceeded) // deadline has already passed
                return c, func() { c.cancel(false, Canceled) }
        }
        c.mu.Lock()
        defer c.mu.Unlock()
        // 再做个检查, 证明这个ctx没被取消
        // 如果ctx被cancel了, c.err就会有值
        if c.err == nil {
                c.timer = time.AfterFunc(dur, func() {
                        c.cancel(true, DeadlineExceeded)
                })  
        }   
        return c, func() { c.cancel(true, Canceled) }
}
```
## context最佳实践
1. 传压测数据来看WithValue, 如果Value很多, 可以做个聚合. 减少创建链表中的节点数
```console
goos: darwin
goarch: amd64
pkg: test
cpu: Intel(R) Core(TM) i7-1068NG7 CPU @ 2.30GHz
BenchmarkWithValue-8       	 2927892	       420.0 ns/op
BenchmarkWithValue_One-8   	14081560	        85.99 ns/op
PASS
ok  	test	3.062s

```
2. 以前的context是不建议内嵌的. 现在没这个限制, 为了保持代码兼容性, 可以继续保持这一点
