package repository

import "container/heap"

type IntHeap []int

func (h IntHeap) Len() int           { return len(h) }
func (h IntHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h IntHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func NewHeap(data []int) *IntHeap {
	h := &IntHeap{}
	heap.Init(h)
	for _, v := range data {
		h.Push(v)
	}
	return h
}

func (h *IntHeap) Push(x any) {
	*h = append(*h, x.(int))
}

func (h *IntHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *IntHeap) Sort() []int {
	heap.Init(h)
	return Sort(*h)
}

func Sort(arr []int) []int {

	if len(arr) < 2 {
		return arr
	}
	pivot := arr[0]
	var less []int
	var greater []int
	for i := 1; i < len(arr); i++ {
		if arr[i] >= pivot {
			less = append(less, arr[i])
		} else {
			greater = append(greater, arr[i])
		}
	}
	return append(append(Sort(less), pivot), Sort(greater)...)
}
