// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/vmihailenco/msgpack/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	flavor             = "global-ratelimit"
	administrativePort = 8800
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
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	rpaasInstanceName, exists := pod.Labels["rpaas.extensions.tsuru.io/instance-name"]
	if !exists {
		return ctrl.Result{}, nil
	}

	if pod.Status.PodIP == "" {
		r.Log.Info("ip not found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{RequeueAfter: time.Second * 1}, nil
	}

	var rpaasInstance rpaasOperatorv1alpha1.RpaasInstance
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: rpaasInstanceName}, &rpaasInstance); err != nil {
		r.Log.Error(err, "Failed to get rpaas instance", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !slices.Contains(rpaasInstance.Spec.Flavors, flavor) {
		r.Log.Info("Rpaas Instance does not have expected flavor", "namespace", req.Namespace, "name", req.Name, "flavor", flavor)
		return ctrl.Result{}, nil
	}

	zoneNames, err := r.getNginxRateLimitingZones(&pod) // TODO: check errors
	if err != nil {
		r.Log.Error(err, "Failed to get rate limiting zones", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, err
	}

	if len(zoneNames) == 0 {
		r.Log.Info("no rate limiting zones found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	rpaasInstanceWorker := r.ManagerGoroutine.Start(rpaasInstanceName, manager.NewRpaasInstanceSyncWorker(rpaasInstanceName, zoneNames))
	rpaasInstanceWorker.RpaasInstanceSignals.StartRpaasPodWorker <- [2]string{pod.Name, pod.Status.PodIP}
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
