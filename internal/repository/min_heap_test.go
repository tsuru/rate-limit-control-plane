package repository

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMinHeap(t *testing.T) {
	t.Run("TopKByExcess", func(t *testing.T) {
		assert := assert.New(t)

		data := []Data{
			{Key: "1", Zone: "one", Last: 1622547800, Excess: 10},
			{Key: "2", Zone: "one", Last: 1622547801, Excess: 20},
			{Key: "3", Zone: "one", Last: 1622547802, Excess: 5},
			{Key: "4", Zone: "one", Last: 1622547803, Excess: 15},
		}
		topK := TopKByExcess(data, 2)
		fmt.Println("Top K by Excess:", topK)
		assert.Len(topK, 2)
		assert.Contains(topK, Data{Key: "2", Zone: "one", Last: 1622547801, Excess: 20})
		assert.Contains(topK, Data{Key: "4", Zone: "one", Last: 1622547803, Excess: 15})
	})

	t.Run("TopKByExcess with less data than k", func(t *testing.T) {
		assert := assert.New(t)
		data := make([]Data, 1_000_000)
		now := time.Now().Unix()
		for i := 0; i < len(data); i++ {
			data[i] = Data{
				Key:    strconv.Itoa(i + 1),
				Zone:   "one",
				Last:   rand.Int63n(now),
				Excess: rand.Int63n(100),
			}
		}
		data[87] = Data{Key: "_87", Zone: "one", Last: now, Excess: 110}
		data[780_090] = Data{Key: "_123", Zone: "one", Last: now, Excess: 120}
		topK := TopKByExcess(data, 2)
		assert.Len(topK, 2)
		assert.Contains(topK, Data{Key: "_123", Zone: "one", Last: now, Excess: 120})
		assert.Contains(topK, Data{Key: "_87", Zone: "one", Last: now, Excess: 110})
	})
}

func BenchmarkSort(b *testing.B) {
	data := make([]Data, 1_000_000)
	now := time.Now().Unix()
	for i := 0; i < len(data); i++ {
		data[i] = Data{
			Key:    strconv.Itoa(i + 1),
			Last:   rand.Int63n(now),
			Excess: rand.Int63n(100),
		}
	}

	b.Run("TopK", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			TopKByExcess(data, 2)
		}
	})
	b.Run("Sort", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			slices.SortFunc(data, func(a, b Data) int {
				if a.Excess < b.Excess {
					return 1
				}
				if a.Excess > b.Excess {
					return -1
				}
				return 0
			})
		}
	})
}
