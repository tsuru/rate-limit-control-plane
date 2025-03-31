// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
	"github.com/vmihailenco/msgpack/v5"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type RateLimitControllerReconcile struct {
	client.Client
	Log              logr.Logger
	ManagerGoroutine *manager.GoroutineManager
	Namespace        string
	PodCache         sync.Map
}

func (r *RateLimitControllerReconcile) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != "rpaasv2" {
		return ctrl.Result{}, nil
	}
	var pod corev1.Pod
	if err := r.Client.Get(ctx, req.NamespacedName, &pod); err != nil {
		if lastIP, ok := r.PodCache.Load(req.NamespacedName.String()); ok {
			r.ManagerGoroutine.Stop(lastIP.(string))
			r.Log.Info("pod deleted", "namespace", req.Namespace, "name", req.Name, "lastPodIP", lastIP)
			r.PodCache.Delete(req.NamespacedName.String())
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	_, exists := pod.Labels["rpaas.extensions.tsuru.io/instance-name"]
	if !exists {
		return ctrl.Result{}, nil
	}
	// TODO: get flavor from rpaasCRD
	if pod.Status.PodIP == "" {
		r.Log.Info("ip not found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{RequeueAfter: time.Second * 1}, nil
	}
	r.PodCache.Store(req.NamespacedName.String(), pod.Status.PodIP)
	zonesNames, err := r.getNginxRateLimitingZones(&pod) // TODO: check errors
	if err != nil {
		r.Log.Error(err, "Failed to get rate limiting zones", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, err
	}
	if len(zonesNames) == 0 {
		r.Log.Info("no rate limiting zones found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}
	r.ManagerGoroutine.Start(pod.Status.PodIP, r.getNginxRateLimitZoneEntries(&pod), zonesNames)
	r.Log.Info("pod started", "namespace", req.Namespace, "name", req.Name, "podIP", pod.Status.PodIP)
	return ctrl.Result{}, nil
}

func (r *RateLimitControllerReconcile) getNginxRateLimitZoneEntries(nginxInstance *corev1.Pod) func(zone string) (manager.Zone, error) {
	return func(zone string) (manager.Zone, error) {
		endpoint := fmt.Sprintf("http://%s:%d/%s/%s", nginxInstance.Status.PodIP, administrativePort, "rate-limit", zone)
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return manager.Zone{}, err
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			return manager.Zone{}, err
		}
		defer response.Body.Close()
		var rateLimitEntries []manager.RateLimitEntry
		decoder := msgpack.NewDecoder(response.Body)
		for {
			var message manager.RateLimitEntry
			if err := decoder.Decode(&message); err != nil {
				if err == io.EOF {
					break
				}
				return manager.Zone{}, err
			}
			rateLimitEntries = append(rateLimitEntries, message)
		}
		return manager.Zone{
			Name:             zone,
			RateLimitEntries: rateLimitEntries,
		}, nil
	}
}

func (c *RateLimitControllerReconcile) getNginxRateLimitingZones(nginxInstance *corev1.Pod) ([]string, error) {
	zones := []string{}
	endpoint := fmt.Sprintf("http://%s:%d/%s", nginxInstance.Status.PodIP, administrativePort, "rate-limit")
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	decoder := msgpack.NewDecoder(response.Body)
	if err := decoder.Decode(&zones); err != nil {
		return nil, err
	}
	return zones, nil
}

func (r *RateLimitControllerReconcile) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			DeleteFunc:  func(e event.DeleteEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		}).
		Complete(r)
}
