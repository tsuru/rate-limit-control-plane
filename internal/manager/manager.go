// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"fmt"
	"sync"
	"time"
)

type Params struct {
	stop  chan bool
	work  chan bool
	value float64
}

type GoroutineManager struct {
	mu    sync.Mutex
	tasks map[string]Params
}

type GoroutineManagerInterface interface {
	Start(id string, workFunc func())
	Stop(id string)
	Run(id string)
}

var _ GoroutineManagerInterface = &GoroutineManager{}

func NewGoroutine() *GoroutineManager {
	return &GoroutineManager{
		tasks: map[string]Params{},
	}
}

func (gm *GoroutineManager) Start(id string, workFunc func()) {
	gm.mu.Lock()
	if _, exists := gm.tasks[id]; exists {
		gm.mu.Unlock()
		return
	}
	stop := make(chan bool, 1)
	work := make(chan bool, 1)
	gm.tasks[id] = Params{
		stop: stop,
		work: work,
	}
	gm.mu.Unlock()

	go func() {
		for {
			select {
			case <-stop:
				fmt.Printf("end goroutines %s\n", id)
				return
			case <-work:
				workFunc()
			default:
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func (gm *GoroutineManager) Stop(id string) {
	gm.mu.Lock()
	if param, exists := gm.tasks[id]; exists {
		param.stop <- true
		close(param.stop)
		delete(gm.tasks, id)
		fmt.Printf("stopping %s\n", id)
	} else {
		fmt.Printf("not found %s\n", id)
	}
	gm.mu.Unlock()
}

func (gm *GoroutineManager) Run(id string) {
	gm.mu.Lock()
	if param, exists := gm.tasks[id]; exists {
		param.work <- true
	}
	gm.mu.Unlock()
}

func (gm *GoroutineManager) ListTasks() {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	for id := range gm.tasks {
		fmt.Printf("* ID: %s\n", id)
	}
}
