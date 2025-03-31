// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"fmt"
	"time"
)

type GoroutineManagerInterface interface {
	Start(id string, workFunc workFunc, zones []string)
	Stop(id string)
	Run(id string)
}

var _ GoroutineManagerInterface = &GoroutineManager{}

func NewGoroutine() *GoroutineManager {
	return &GoroutineManager{
		tasks: map[string]Params{},
	}
}

func (gm *GoroutineManager) Start(id string, workFunc workFunc, zones []string) {
	gm.mu.Lock()
	if _, exists := gm.tasks[id]; exists {
		gm.mu.Unlock()
		return
	}
	stop := make(chan bool, 1)
	work := make(chan bool, 1)
	params := Params{
		stop: stop,
		work: work,
	}
	for _, zoneName := range zones {
		params.zones = append(params.zones, Zone{
			Name:             zoneName,
			RateLimitEntries: []RateLimitEntry{},
		})
	}
	gm.tasks[id] = params
	gm.mu.Unlock()

	go func() {
		for {
			select {
			case <-stop:
				fmt.Printf("end goroutines %s\n", id)
				return
			case <-work:
				for _, zone := range params.zones {
					result, err := workFunc(zone.Name)
					if err != nil {
						fmt.Printf("error %s\n", err)
						continue
					}
					for i, z := range params.zones {
						if z.Name == zone.Name {
							params.zones[i].RateLimitEntries = result.RateLimitEntries
						}
					}
				}
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
	for ip, params := range gm.tasks {
		message := ""
		message += fmt.Sprintf("* ID: %s - ", ip)
		for _, zone := range params.zones {
			message += fmt.Sprintf(" * Zone: %s - - %v", zone.Name, zone.RateLimitEntries)
		}
		message += "\n"
		fmt.Println(message)
	}
}

func (gm *GoroutineManager) GetTask() []string {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	ips := []string{}
	for id := range gm.tasks {
		ips = append(ips, id)
	}
	return ips
}
