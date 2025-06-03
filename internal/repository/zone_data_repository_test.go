package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

func TestNewRpaasZoneDataRepository(t *testing.T) {
	t.Run("should create a new RpaasZoneDataRepository", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(r)
		assert.NotNil(ch)
	})

	t.Run("should initialize Data map", func(t *testing.T) {
		assert := assert.New(t)
		r, _ := NewRpaasZoneDataRepository()
		assert.NotNil(r.Data)
		assert.Empty(r.Data)
	})
	t.Run("should return a channel for reading RpaasZoneData", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547800,
						Excess: 10,
					},
				},
			}},
		})
		b, ok := r.GetRpaasZoneData("test-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 1)
		assert.Equal("test-key:test-zone", rpaasZoneData[0].ID)
		assert.Equal(int64(1622547800), rpaasZoneData[0].Last)
		assert.Equal(int64(10), rpaasZoneData[0].Excess)
	})

	t.Run("should handle empty RpaasZoneData", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "empty-rpaas",
			Data:      []ratelimit.Zone{},
		})
		b, ok := r.GetRpaasZoneData("empty-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 0)
	})

	t.Run("should return false for non-existing RpaasName", func(t *testing.T) {
		assert := assert.New(t)
		r, _ := NewRpaasZoneDataRepository()
		_, exists := r.GetRpaasZoneData("non-existing-rpaas")
		assert.False(exists)
	})

	t.Run("should list instances", func(t *testing.T) {
		assert := assert.New(t)
		r, _ := NewRpaasZoneDataRepository()
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "instance1",
		})
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "instance2",
		})
		instances := r.ListInstances()
		assert.Len(instances, 2)
		assert.Contains(instances, "instance1")
		assert.Contains(instances, "instance2")
	})

	t.Run("should handle multiple entries for the same key", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547800,
						Excess: 10,
					},
				},
			}},
		})
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547801,
						Excess: 27,
					},
				},
			}},
		})
		b, ok := r.GetRpaasZoneData("test-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 1)
		assert.Equal("test-key:test-zone", rpaasZoneData[0].ID)
		assert.Equal(int64(1622547801), rpaasZoneData[0].Last)
		assert.Equal(int64(27), rpaasZoneData[0].Excess)
	})

	t.Run("should handle multiple zones.", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone-one",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547800,
						Excess: 10,
					},
				},
			}},
		})
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone-two",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547801,
						Excess: 27,
					},
				},
			}},
		})
		b, ok := r.GetRpaasZoneData("test-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 1)
		assert.Equal("test-key:test-zone-two", rpaasZoneData[0].ID)
		assert.Equal(int64(1622547801), rpaasZoneData[0].Last)
		assert.Equal(int64(27), rpaasZoneData[0].Excess)
	})

	t.Run("should handle multiple zones with the same key", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone-one",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547805,
						Excess: 10,
					},
				},
			}},
		})
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone-one",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key-one"),
						Last:   1622547810,
						Excess: 27,
					},
					{
						Key:    []byte("test-key-two"),
						Last:   1622547803,
						Excess: 5,
					},
					{
						Key:    []byte("test-key-three"),
						Last:   1622547800,
						Excess: 10,
					},
				},
			}},
		})
		b, ok := r.GetRpaasZoneData("test-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 3)
		assert.Equal("test-key-one:test-zone-one", rpaasZoneData[0].ID)
		assert.Equal(int64(1622547810), rpaasZoneData[0].Last)
		assert.Equal(int64(27), rpaasZoneData[0].Excess)

		assert.Equal("test-key-three:test-zone-one", rpaasZoneData[1].ID)
		assert.Equal(int64(1622547800), rpaasZoneData[1].Last)
		assert.Equal(int64(10), rpaasZoneData[1].Excess)

		assert.Equal("test-key-two:test-zone-one", rpaasZoneData[2].ID)
		assert.Equal(int64(1622547803), rpaasZoneData[2].Last)
		assert.Equal(int64(5), rpaasZoneData[2].Excess)
	})

	t.Run("should handle multiple zones with different keys", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone-one",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547805,
						Excess: 10,
					},
				},
			}},
		})
		r.insert(ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{
				{
					Name: "test-zone-one",
					RateLimitEntries: []ratelimit.RateLimitEntry{
						{
							Key:    []byte("test-key-one"),
							Last:   1622547810,
							Excess: 27,
						},
						{
							Key:    []byte("test-key-two"),
							Last:   1622547803,
							Excess: 5,
						},
					},
				},
				{
					Name: "test-zone-two",
					RateLimitEntries: []ratelimit.RateLimitEntry{
						{
							Key:    []byte("test-key-three"),
							Last:   1622547800,
							Excess: 10,
						},
					},
				},
			},
		})
		b, ok := r.GetRpaasZoneData("test-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 3)
		assert.Equal("test-key-one:test-zone-one", rpaasZoneData[0].ID)
		assert.Equal(int64(1622547810), rpaasZoneData[0].Last)
		assert.Equal(int64(27), rpaasZoneData[0].Excess)

		assert.Equal("test-key-three:test-zone-two", rpaasZoneData[1].ID)
		assert.Equal(int64(1622547800), rpaasZoneData[1].Last)
		assert.Equal(int64(10), rpaasZoneData[1].Excess)

		assert.Equal("test-key-two:test-zone-one", rpaasZoneData[2].ID)
		assert.Equal(int64(1622547803), rpaasZoneData[2].Last)
		assert.Equal(int64(5), rpaasZoneData[2].Excess)
	})

	t.Run("should handle multiple zones with different keys with chan", func(t *testing.T) {
		assert := assert.New(t)
		r, ch := NewRpaasZoneDataRepository()
		assert.NotNil(ch)
		ch <- ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{{
				Name: "test-zone-one",
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{
						Key:    []byte("test-key"),
						Last:   1622547805,
						Excess: 10,
					},
				},
			}},
		}
		ch <- ratelimit.RpaasZoneData{
			RpaasName: "test-rpaas",
			Data: []ratelimit.Zone{
				{
					Name: "test-zone-one",
					RateLimitEntries: []ratelimit.RateLimitEntry{
						{
							Key:    []byte("test-key-one"),
							Last:   1622547810,
							Excess: 27,
						},
						{
							Key:    []byte("test-key-two"),
							Last:   1622547803,
							Excess: 5,
						},
					},
				},
				{
					Name: "test-zone-two",
					RateLimitEntries: []ratelimit.RateLimitEntry{
						{
							Key:    []byte("test-key-three"),
							Last:   1622547800,
							Excess: 10,
						},
					},
				},
			},
		}
		time.Sleep(100 * time.Millisecond) // Wait for the reader to process the data
		b, ok := r.GetRpaasZoneData("test-rpaas")
		assert.True(ok)
		rpaasZoneData := []Data{}
		err := json.Unmarshal(b, &rpaasZoneData)
		assert.Nil(err)
		assert.Len(rpaasZoneData, 3)
		assert.Equal("test-key-one:test-zone-one", rpaasZoneData[0].ID)
		assert.Equal(int64(1622547810), rpaasZoneData[0].Last)
		assert.Equal(int64(27), rpaasZoneData[0].Excess)

		assert.Equal("test-key-three:test-zone-two", rpaasZoneData[1].ID)
		assert.Equal(int64(1622547800), rpaasZoneData[1].Last)
		assert.Equal(int64(10), rpaasZoneData[1].Excess)

		assert.Equal("test-key-two:test-zone-one", rpaasZoneData[2].ID)
		assert.Equal(int64(1622547803), rpaasZoneData[2].Last)
		assert.Equal(int64(5), rpaasZoneData[2].Excess)
	})

}
