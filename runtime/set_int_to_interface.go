package main

var i interface{}

func main() {
	var y = 888
	i = y

	z := i.(int)
	println(i, z)
}
