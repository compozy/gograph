package b

// Note: This would create a circular dependency if uncommented
// import "github.com/test/circular/pkg/a"

func FuncB() string {
	// return "B: " + a.GetA() // Would cause circular dep
	return "B: standalone"
}

func GetB() string {
	return "Value from B"
}