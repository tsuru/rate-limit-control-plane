// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"fmt"
	"time"
)

func NewGoroutine() *GoroutineManager {
	gm := GoroutineManager{
		tasks: map[string]*RpaasInstanceSyncWorker{},
	}
	return &gm
}

// func FullZoneSync(gm *GoroutineManager) {
// 	for {
// 		time.Sleep(5 * time.Second)
// 		gm.fullZone = map[FullZoneKey]RateLimitEntry{}
// 		for _, task := range gm.tasks {
// 			for _, zone := range task.zones {
// 				for _, entity := range zone.RateLimitEntries {
// 					sEnc := b64.StdEncoding.EncodeToString([]byte(entity.Key))
// 					hashID := FullZoneKey{
// 						Zone: zone.Name,
// 						Key:  sEnc,
// 					}
// 					fmt.Println("hashID", hashID)
// 					gm.mu.Lock()
//
// 					if _, exists := gm.fullZone[hashID]; exists {
// 						fullZone := gm.fullZone[hashID]
// 						fullZone.Excess += entity.Excess
// 						fullZone.Last = entity.Last
// 						gm.mu.Unlock()
// 						continue
// 					}
// 					gm.fullZone[hashID] = RateLimitEntry{
// 						Key:    entity.Key,
// 						Last:   entity.Last,
// 						Excess: entity.Excess,
// 					}
// 					gm.mu.Unlock()
// 				}
// 			}
// 		}
// 		for zoneKey, zone := range gm.fullZone {
// 			fmt.Printf("FullZone: %v - zone: %v\n", zoneKey, zone)
// 		}
// 	}
// }

func (gm *GoroutineManager) Start(id string, rpaasInstanceWorker *RpaasInstanceSyncWorker) *RpaasInstanceSyncWorker {
	gm.mu.Lock()
	if worker, exists := gm.tasks[id]; exists {
		gm.mu.Unlock()
		return worker
	}
	stop := make(chan bool, 1)
	gm.tasks[id] = rpaasInstanceWorker
	gm.mu.Unlock()

	go func() {
		for {
			select {
			case <-stop:
				fmt.Printf("end goroutines %s\n", id)
				rpaasInstanceWorker.RpaasInstanceSignals.StopChan <- struct{}{}
				return
			default:
				time.Sleep(1 * time.Second)
			}
		}
	}()
	go rpaasInstanceWorker.Work()
	return rpaasInstanceWorker
}

func (gm *GoroutineManager) Stop(id string) {
	gm.mu.Lock()
	if rpaasInstanceWorker, exists := gm.tasks[id]; exists {
		rpaasInstanceWorker.RpaasInstanceSignals.StopChan <- struct{}{}
		close(rpaasInstanceWorker.RpaasInstanceSignals.StopChan)
		delete(gm.tasks, id)
		fmt.Printf("stopping %s\n", id)
	} else {
		fmt.Printf("not found %s\n", id)
	}
	gm.mu.Unlock()
}

// func (gm *GoroutineManager) ListTasks() {
// 	gm.mu.Lock()
// 	defer gm.mu.Unlock()
// 	for ip, params := range gm.tasks {
// 		message := ""
// 		message += fmt.Sprintf("* ID: %s - ", ip)
// 		for _, zone := range params.zones {
// 			message += fmt.Sprintf(" * Zone: %s - - %v", zone.Name, zone.RateLimitEntries)
// 		}
// 		message += "\n"
// 		fmt.Println(message)
// 		for _, fullZone := range gm.fullZone {
// 			fmt.Printf(" * FullZone: %s - %v\n", fullZone.Key, fullZone)
// 		}
// 		fmt.Println("========================================")
// 	}
// }

// func (gm *GoroutineManager) GetTask() []string {
// 	gm.mu.Lock()
// 	defer gm.mu.Unlock()
// 	ips := []string{}
// 	for id := range gm.tasks {
// 		ips = append(ips, id)
// 	}
// 	return ips
// }
