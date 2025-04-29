// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"sync"
)

type GoroutineManager struct {
	mu      sync.Mutex
	workers map[string]Worker
}

func NewGoroutineManager() *GoroutineManager {
	return &GoroutineManager{
		workers: make(map[string]Worker),
	}
}

func (gm *GoroutineManager) AddWorker(worker Worker) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	id := worker.GetID()
	if _, exists := gm.workers[id]; !exists {
		gm.workers[id] = worker
		go worker.Start()
	}
}

func (gm *GoroutineManager) RemoveWorker(id string) bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if worker, exists := gm.workers[id]; exists {
		worker.Stop()
		delete(gm.workers, id)
		return true
	}
	return false
}

func (gm *GoroutineManager) GetWorker(id string) (Worker, bool) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	worker, exists := gm.workers[id]
	return worker, exists
}

func (gm *GoroutineManager) ListWorkerIDs() []string {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	ids := make([]string, 0, len(gm.workers))
	for id := range gm.workers {
		ids = append(ids, id)
	}
	return ids
}

func (gm *GoroutineManager) ForEachWorker(f func(Worker)) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	for _, worker := range gm.workers {
		f(worker)
	}
}
