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
			{ID: "1", Last: 1622547800, Excess: 10},
			{ID: "2", Last: 1622547801, Excess: 20},
			{ID: "3", Last: 1622547802, Excess: 5},
			{ID: "4", Last: 1622547803, Excess: 15},
		}
		topK := TopKByExcess(data, 2)
		fmt.Println("Top K by Excess:", topK)
		assert.Len(topK, 2)
		assert.Contains(topK, Data{ID: "2", Last: 1622547801, Excess: 20})
		assert.Contains(topK, Data{ID: "4", Last: 1622547803, Excess: 15})
	})

	t.Run("TopKByExcess with less data than k", func(t *testing.T) {
		assert := assert.New(t)
		data := make([]Data, 1_000_000)
		now := time.Now().Unix()
		for i := 0; i < len(data); i++ {
			data[i] = Data{
				ID:     strconv.Itoa(i + 1),
				Last:   rand.Int63n(now),
				Excess: rand.Int63n(100),
			}
		}
		data[87] = Data{ID: "_87", Last: now, Excess: 110}
		data[780_090] = Data{ID: "_123", Last: now, Excess: 120}
		topK := TopKByExcess(data, 2)
		assert.Len(topK, 2)
		assert.Contains(topK, Data{ID: "_123", Last: now, Excess: 120})
		assert.Contains(topK, Data{ID: "_87", Last: now, Excess: 110})
	})
}

func BenchmarkSort(b *testing.B) {
	data := make([]Data, 1_000_000)
	now := time.Now().Unix()
	for i := 0; i < len(data); i++ {
		data[i] = Data{
			ID:     strconv.Itoa(i + 1),
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
