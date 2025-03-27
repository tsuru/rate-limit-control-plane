package manager

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagerGoRoutines(t *testing.T) {

	t.Run("should create go routines", func(t *testing.T) {
		assert := assert.New(t)
		manager := NewGoroutineManager()
		assert.Len(manager.tasks, 0)

		manager.Start("1", func() {})
		assert.Len(manager.tasks, 1)
		manager.Start("1", func() {})
		assert.Len(manager.tasks, 1)

		manager.Start("2", func() {})
		assert.Len(manager.tasks, 2)
	})

	t.Run("should stop go routines", func(t *testing.T) {
		assert := assert.New(t)
		manager := NewGoroutineManager()
		manager.Start("1", func() {})
		manager.Start("2", func() {})
		assert.Len(manager.tasks, 2)
		manager.Stop("1")

		assert.Len(manager.tasks, 1)
		_, ok := manager.tasks["2"]
		assert.True(ok)
		_, ok = manager.tasks["1"]
		assert.False(ok)
	})

	t.Run("should execute work", func(t *testing.T) {
		assert := assert.New(t)
		countWork := 0
		ch := make(chan bool)
		work := func() {
			countWork++
			fmt.Println("*")
			<-ch
		}
		manager := NewGoroutineManager()
		manager.Start("1", work)
		manager.Run("1")
		ch <- true
		assert.Equal(1, countWork)
		assert.Len(manager.tasks, 1)
	})

	t.Run("should execute work with two start", func(t *testing.T) {
		assert := assert.New(t)
		countWork := 0
		ch := make(chan bool)
		work := func() {
			countWork++
			fmt.Println("*")
			<-ch
		}
		manager := NewGoroutineManager()
		manager.Start("1", work)
		manager.Start("1", work)
		manager.Run("1")
		ch <- true
		assert.Equal(1, countWork)
		assert.Len(manager.tasks, 1)
	})

	t.Run("should execute two work with", func(t *testing.T) {
		assert := assert.New(t)
		countWork := 0
		ch := make(chan bool)
		work := func() {
			countWork++
			fmt.Println("*")
			<-ch
		}
		manager := NewGoroutineManager()
		manager.Start("1", work)
		manager.Run("1")
		ch <- true
		assert.Equal(1, countWork)
		assert.Len(manager.tasks, 1)

		countWork2 := 0
		ch2 := make(chan bool)
		work2 := func() {
			countWork2++
			<-ch2
		}
		manager.Start("2", work2)
		manager.Run("2")
		ch2 <- true
		assert.Equal(1, countWork2)
		assert.Len(manager.tasks, 2)
	})
}
