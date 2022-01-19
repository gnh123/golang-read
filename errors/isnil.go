package main

import "fmt"

type x struct{}

func main() {
	var y interface{}
	y = (*x)(nil)

	fmt.Println(y == nil)
}
