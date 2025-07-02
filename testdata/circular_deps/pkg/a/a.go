package a

import "github.com/test/circular/pkg/b"

func FuncA() string {
	return "A: " + b.GetB()
}

func GetA() string {
	return "Value from A"
}