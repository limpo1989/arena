package main

import (
	"fmt"

	"github.com/limpo1989/arena"
)

func main() {
	ar := arena.NewArena()
	defer ar.Reset()

	// Allocate primitives
	num := arena.New[int](ar)
	*num = 42
	fmt.Println("print num:", *num)

	// Allocate slices
	slice := arena.NewSlice[string](ar, 0, 10)
	slice = arena.Append(ar, slice, "hello", "world")
	fmt.Println("print slice:", slice)

	// Use vectors
	vec := arena.NewVector[int](ar, 8)
	vec.Append(1, 2, 3)
	fmt.Println("print vec:", vec.At(0), vec.At(1), vec.At(2))
}
