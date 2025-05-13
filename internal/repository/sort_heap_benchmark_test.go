package repository

import (
	"testing"
)

func BenchmarkWithHeap(b *testing.B) {
	value := []int{}
	for i := 0; i < 50_000; i++ {
		value = append(value, i)
	}

	for i := 0; i < b.N; i++ {
		heap := NewHeap(value)
		heap.Sort()
	}

}

func BenchmarkWithoutHeap(b *testing.B) {
	value := []int{}
	for i := 0; i < 50_000; i++ {
		value = append(value, i)
	}
	for i := 0; i < b.N; i++ {
		Sort(value)
	}
}
