package repository

import (
	"container/heap"
)

type MinHeapData []Data

func (h MinHeapData) Len() int            { return len(h) }
func (h MinHeapData) Less(i, j int) bool  { return h[i].Excess < h[j].Excess }
func (h MinHeapData) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *MinHeapData) Push(x interface{}) { *h = append(*h, x.(Data)) }

func (h *MinHeapData) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func TopKByExcess(data []Data, k int) []Data {
	if len(data) <= k {
		return data
	}
	h := &MinHeapData{}
	heap.Init(h)
	for _, d := range data {
		if h.Len() < k {
			heap.Push(h, d)
		} else if d.Excess > (*h)[0].Excess {
			heap.Pop(h)
			heap.Push(h, d)
		}
	}
	result := make([]Data, h.Len())
	copy(result, *h)
	return result
}
