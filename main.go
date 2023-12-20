package main

import "fmt"

var version string
var revision string
var build string

func main() {
	add(1, 2)
	_version()
}

func _version() {
	fmt.Println(version, revision, build)
}

func add(a, b int) int {
	return a + b
}
