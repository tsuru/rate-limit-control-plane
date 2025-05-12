// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/vmihailenco/msgpack/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/tsuru/rate-limit-control-plane/internal/logger"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
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
	Notify           chan ratelimit.RpaasZoneData
}

func (r *RateLimitControllerReconcile) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if client.IgnoreNotFound(err) == nil {
			r.Log.Info("pod not found", "namespace", req.Namespace, "name", req.Name)
			parts := strings.Split(req.Name, "-")
			if len(parts) < 3 {
				r.Log.Info("pod name does not match expected pattern", "name", req.Name)
				return ctrl.Result{}, nil
			}
			instanceName := strings.Join(parts[:len(parts)-2], "-")
			worker, ok := r.ManagerGoroutine.GetWorker(instanceName)
			if !ok {
				r.Log.Info("worker not found", "instanceName", instanceName, "reuestName", req.Name)
				return ctrl.Result{}, nil
			}
			rpaasInstanceSyncWorker, ok := worker.(*manager.RpaasInstanceSyncWorker)
			if !ok {
				r.Log.Info("worker is not RpaasInstanceSyncWorker", "instanceName", instanceName, "requestName", req.Name)
				return ctrl.Result{}, nil
			}
			if err := rpaasInstanceSyncWorker.RemovePodWorker(req.Name); err != nil {
				r.Log.Info("RpaasPodWorker not found", "instanceName", instanceName, "requestName", req.Name)
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get pod", "namespace", req.Namespace, "name", req.Name)
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

	zoneNames, err := r.getNginxRateLimitingZones(&pod)
	if err != nil {
		r.Log.Error(err, "Failed to get rate limiting zones", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	if len(zoneNames) == 0 {
		r.Log.Info("no rate limiting zones found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}
	worker, exists := r.ManagerGoroutine.GetWorker(rpaasInstanceName)
	if !exists {
		instanceLogger := logger.NewLogger(map[string]string{"emitter": "rate-limit-control-plane"}, os.Stdout)
		worker = manager.NewRpaasInstanceSyncWorker(rpaasInstanceName, zoneNames, instanceLogger, r.Notify)
		r.ManagerGoroutine.AddWorker(worker)
	}
	// convert worker to RpaasInstanceSyncWorker
	rpaasInstanceSyncWorker, ok := worker.(*manager.RpaasInstanceSyncWorker)
	if ok {
		rpaasInstanceSyncWorker.AddPodWorker(pod.Status.PodIP, pod.Name)
	}

	r.Log.Info("pod started", "namespace", req.Namespace, "name", req.Name, "podIP", pod.Status.PodIP)
	return ctrl.Result{}, nil
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
