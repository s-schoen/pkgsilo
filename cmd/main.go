package main

import "fmt"

func main() {
	fmt.Println(greeting("world"))
}

func greeting(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
