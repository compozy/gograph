package main

import (
	"fmt"
	"github.com/test/simple/pkg/utils"
)

func main() {
	fmt.Println("Hello, World!")
	result := process("test")
	fmt.Printf("Result: %s\n", result)
	utils.Helper()
}

func process(input string) string {
	return utils.Transform(input)
}