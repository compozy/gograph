package utils

import "strings"

// Helper is a utility function
func Helper() {
	println("Helper called")
}

// Transform transforms the input string
func Transform(s string) string {
	return strings.ToUpper(s)
}

// Calculate performs a calculation
func Calculate(a, b int) int {
	return a + b
}