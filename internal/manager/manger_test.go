// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type work struct {
	ID string
}

func (work) Start() {
}

func (work) Stop() {
}

func (w work) GetID() string {
	return w.ID
}

func (w *work) SetID(id string) {
	w.ID = id
}

func TestManagerGoRoutines(t *testing.T) {
	t.Run("should create go routines", func(t *testing.T) {
		assert := assert.New(t)
		manager := NewGoroutineManager()
		assert.Len(manager.workers, 0)

		w := work{ID: "1"}
		w2 := work{ID: "2"}

		manager.AddWorker(w)
		assert.Len(manager.workers, 1)

		manager.AddWorker(w2)
		assert.Len(manager.workers, 2)
	})

	t.Run("should remove work", func(t *testing.T) {
		assert := assert.New(t)
		manager := NewGoroutineManager()
		w1 := work{ID: "1"}
		w2 := work{ID: "2"}
		manager.AddWorker(w1)
		assert.Len(manager.workers, 1)
		manager.AddWorker(w2)
		assert.Len(manager.workers, 2)
		manager.RemoveWorker("1")
		assert.Len(manager.workers, 1)
		ids := manager.ListWorkerIDs()
		assert.Equal("2", ids[0])
	})
}
