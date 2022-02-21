```v1.17.6```

## 零、前言```strings``` 包
strings包下面也是高频函数. 与string打交道, 就需要用到, 比如统计某个字符串出现的次数. 替换某些字符串


## 一、```strings.Count```
1. 通过Index函数找substr, 找到则返回位置
2. 记录找到的个数(n++), 修改s, 跳过已搜索过的位置
```go
// Count counts the number of non-overlapping instances of substr in s.
// If substr is an empty string, Count returns 1 + the number of Unicode code points in s.
func Count(s, substr string) int {
        // special case
        if len(substr) == 0 {
                return utf8.RuneCountInString(s) + 1
        }
        // 统计个数, substr为1, 那就是单字节的个数
        if len(substr) == 1 {
                return bytealg.CountString(s, substr[0])
        }
        n := 0
        for {
                // 见1
                i := Index(s, substr)
                if i == -1 {
                        return n
                }
                // 见2
                n++
                // 见2
                s = s[i+len(substr):]
        }
}
```

## 二、```strings.Replace```
1. 如果潜换的新值等于旧值, 或者n为0, 那就不替换
2. 先统计要被替换的字符有多少个, Replace的n为负值时, 全局替换, 或者要替换的个数>= 实际的old串的个数, 修改下替换个数
3. 计算新串需要的内部空间 len(s) + n *(len(new) - len(old))
```
// Replace returns a copy of the string s with the first n
// non-overlapping instances of old replaced by new.
// If old is empty, it matches at the beginning of the string
// and after each UTF-8 sequence, yielding up to k+1 replacements
// for a k-rune string.
// If n < 0, there is no limit on the number of replacements.
func Replace(s, old, new string, n int) string {
        // 见1
        if old == new || n == 0 {
                return s // avoid allocation
        }

        // 见2
        // Compute number of replacements.
        if m := Count(s, old); m == 0 {
                return s // avoid allocation
                // 见2
        } else if n < 0 || m < n {
                n = m
        }

        // Apply replacements to buffer.
        var b Builder
        // 见3
        b.Grow(len(s) + n*(len(new)-len(old)))
        start := 0
        for i := 0; i < n; i++ {
                j := start
                if len(old) == 0 {
                        if i > 0 {
                                _, wid := utf8.DecodeRuneInString(s[start:])
                                j += wid
                        }
                } else {
                        j += Index(s[start:], old)
                }
                b.WriteString(s[start:j])
                b.WriteString(new)
                start = j + len(old)
        }
        b.WriteString(s[start:])
        return b.String()
}

// ReplaceAll returns a copy of the string s with all
// non-overlapping instances of old replaced by new.
// If old is empty, it matches at the beginning of the string
// and after each UTF-8 sequence, yielding up to k+1 replacements
// for a k-rune string.
func ReplaceAll(s, old, new string) string {
        return Replace(s, old, new, -1)
}
```
