package main

import (
	"fmt"
	"math/rand"
	"strings"
	//"golang.org/x/tour/wc"
	//"golang.org/x/tour/pic"
)

func WordCount(s string) map[string]int {
	words := strings.Fields(s)
	counts := make(map[string]int)
	for _, key := range words {
		counts[key]++
	}

	return counts
}

func fibonacci() func() int {
	prev, prev2 := 0, 1

	return func() int {
		temp := prev
		prev = prev2
		prev2 = temp + prev2
		return temp
	}
}

func ifStateMents() {
	x := rand.Intn(999)

	if v := rand.Intn(999); v < x {
		fmt.Println("X is greater")
	}

}

func Pic(dx, dy int) [][]uint8 {
	arr := make([][]uint8, dy)
	fmt.Println(len(arr))
	for i := 0; i < dx; i++ {
		arr[i] = make([]uint8, dx)
		for j := range arr[i] {
			arr[i][j] = uint8(rand.Intn(255))
		}
	}

	return arr
}

func main() {
	// pic.Show(Pic)
	// wc.Test(WordCount)
	ifStateMents()
	f := fibonacci()
	for i := 0; i < 20; i++ {
		fmt.Println(f())
	}
}
