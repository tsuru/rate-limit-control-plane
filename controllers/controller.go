// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
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
	if pod.Status.PodIP == "" {
		r.Log.Info("ip not found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{RequeueAfter: time.Second * 1}, nil
	}
	r.PodCache.Store(req.NamespacedName.String(), pod.Status.PodIP)
	r.ManagerGoroutine.Start(pod.Status.PodIP, func() {
		fmt.Println("pod IP: ", pod.Status.PodIP)
	})
	r.Log.Info("pod started", "namespace", req.Namespace, "name", req.Name, "podIP", pod.Status.PodIP)
	return ctrl.Result{}, nil
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
