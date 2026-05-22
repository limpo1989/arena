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
	fmt.Println("num:", *num)

	// Allocate slices
	slice := arena.NewSlice[string](ar, 0, 10)
	slice = arena.Append(ar, slice, "hello", "world")
	fmt.Println("slice:", slice)

	// Deep copy a struct into arena
	type Player struct {
		Name  string
		Level int
	}
	p := arena.DeepCopy(ar, Player{Name: "Alice", Level: 42})
	fmt.Println("player:", p.Name, "level:", p.Level)

	// Use Vector
	vec := arena.NewVector[int](ar, 8)
	vec.Append(10, 20, 30)
	fmt.Print("vector:")
	for i, v := range vec.All() {
		fmt.Print(" [", i, "]=", v)
	}
	fmt.Println()

	// Use Map
	m := arena.NewMap[string, int](ar, 8)
	m.Put("x", 100)
	m.Put("y", 200)
	if v, ok := m.Get("x"); ok {
		fmt.Println("map[x]:", v)
	}
	fmt.Print("map iterate:")
	for k, v := range m.All() {
		fmt.Print(" ", k, "=", v)
	}
	fmt.Println()
}
