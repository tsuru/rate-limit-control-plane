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
	"time"

	"github.com/go-logr/logr"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/vmihailenco/msgpack/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/tsuru/rate-limit-control-plane/internal/aggregator"
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

func (r *RateLimitControllerReconcile) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			DeleteFunc:  func(e event.DeleteEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return true },
			GenericFunc: func(e event.GenericEvent) bool { return true },
		}).
		Complete(r)
}

func (r *RateLimitControllerReconcile) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return r.handlePodNotFound(req)
		}
		r.Log.Error(err, "Failed to get pod", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, err
	}

	rpaasInstanceName, exists := pod.Labels["rpaas.extensions.tsuru.io/instance-name"]
	if !exists {
		r.Log.Info("pod does not have rpaas instance label - removing from queue", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	rpaasServiceName, exists := pod.Labels["rpaas.extensions.tsuru.io/service-name"]
	if !exists {
		r.Log.Info("pod does not have rpaas service label - removing from queue", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		r.handlePodStoped(rpaasInstanceName, pod)
	}

	if pod.Status.PodIP == "" {
		r.Log.Info("Pod does not have IP assigned - requeuing", "request", req)
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	if err := r.validateRpaasInstanceFlavor(req, rpaasInstanceName); err != nil {
		r.Log.Error(err, "RpaasInstance does not have expected flavor - removing from queue", "request", req)
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
		rpaasInstanceData := manager.RpaasInstanceData{Instance: rpaasInstanceName, Service: rpaasServiceName}
		worker = manager.NewRpaasInstanceSyncWorker(rpaasInstanceData, zoneNames, instanceLogger, r.Notify, new(aggregator.CompleteAggregator))
		r.ManagerGoroutine.AddWorker(worker)
	}

	rpaasInstanceSyncWorker, ok := worker.(*manager.RpaasInstanceSyncWorker)
	if !ok {
		r.Log.Info("worker is not RpaasInstanceSyncWorker - removing from queue", "instanceName", rpaasInstanceName, "request", req)
		return ctrl.Result{}, nil
	}

	rpaasInstanceSyncWorker.AddPodWorker(pod.Status.PodIP, pod.Name)

	r.Log.Info("pod started", "namespace", req.Namespace, "name", req.Name, "podIP", pod.Status.PodIP)
	return ctrl.Result{}, nil
}

func (r *RateLimitControllerReconcile) handlePodNotFound(req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("pod not found", "req", req)
	instanceName, err := instanceNameFromDeploymentPodName(req.Name)
	if err != nil {
		r.Log.Error(err, "Failed to extract instance name from pod name - removing from queue")
		return ctrl.Result{}, nil
	}

	rpaasInstanceSyncWorker, err := r.getRpaasInstanceWorker(instanceName)
	if err != nil {
		r.Log.Error(err, "Failed to get RpaasInstanceSyncWorker - removing from queue", "instanceName", instanceName, "request", req)
		return ctrl.Result{}, err
	}

	if err := r.removePodWorkerFromRpaasInstanceWorker(rpaasInstanceSyncWorker, req.Name); err != nil {
		r.Log.Error(err, "Failed to remove pod worker from RpaasInstanceSyncWorker - removing from queue", "instanceName", instanceName, "podName", req.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *RateLimitControllerReconcile) handlePodStoped(rpaasInstanceName string, pod corev1.Pod) (ctrl.Result, error) {
	r.Log.Info("pod stopped", "podName", pod.Name, "instanceName", rpaasInstanceName)

	rpaasInstanceSyncWorker, err := r.getRpaasInstanceWorker(rpaasInstanceName)
	if err != nil {
		r.Log.Error(err, "Failed to get RpaasInstanceSyncWorker - removing from queue", "instanceName", rpaasInstanceName, "podPhase", pod.Status.Phase)
		return ctrl.Result{}, nil
	}

	if err := r.removePodWorkerFromRpaasInstanceWorker(rpaasInstanceSyncWorker, pod.Name); err != nil {
		r.Log.Error(err, "Failed to remove pod worker from RpaasInstanceSyncWorker - removing from queue", "instanceName", rpaasInstanceName, "podName", pod.Name, "podPhase", pod.Status.Phase)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *RateLimitControllerReconcile) validateRpaasInstanceFlavor(request ctrl.Request, rpaasInstanceName string) error {
	var rpaasInstance rpaasOperatorv1alpha1.RpaasInstance
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: request.Namespace, Name: rpaasInstanceName}, &rpaasInstance); err != nil {
		return fmt.Errorf("failed to get RpaasInstance %s/%s: %w", request.Namespace, request.Name, err)
	}

	if !slices.Contains(rpaasInstance.Spec.Flavors, flavor) {
		return fmt.Errorf("RpaasInstance %s/%s does not have expected flavor %s", request.Namespace, request.Name, flavor)
	}
	return nil
}

func (r *RateLimitControllerReconcile) getRpaasInstanceWorker(rpaasInstanceName string) (*manager.RpaasInstanceSyncWorker, error) {
	worker, exists := r.ManagerGoroutine.GetWorker(rpaasInstanceName)
	if !exists {
		return nil, fmt.Errorf("worker not found for instance %s", rpaasInstanceName)
	}

	rpaasInstanceSyncWorker, ok := worker.(*manager.RpaasInstanceSyncWorker)
	if !ok {
		return nil, fmt.Errorf("worker is not RpaasInstanceSyncWorker for instance %s", rpaasInstanceName)
	}

	return rpaasInstanceSyncWorker, nil
}

func (r *RateLimitControllerReconcile) removePodWorkerFromRpaasInstanceWorker(rpaasInstanceSyncWorker *manager.RpaasInstanceSyncWorker, podName string) error {
	if err := rpaasInstanceSyncWorker.RemovePodWorker(podName); err != nil {
		return fmt.Errorf("failed to remove pod worker %s from instance %s", podName, rpaasInstanceSyncWorker.GetID())
	}
	if rpaasInstanceSyncWorker.CountWorkers() == 0 {
		if ok := r.ManagerGoroutine.RemoveWorker(rpaasInstanceSyncWorker.GetID()); !ok {
			return fmt.Errorf("failed to remove worker for instance %s", rpaasInstanceSyncWorker.GetID())
		}
	}
	return nil
}

func (r *RateLimitControllerReconcile) getNginxRateLimitingZones(nginxInstance *corev1.Pod) ([]string, error) {
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
	decoder := msgpack.NewDecoder(response.Body)
	if err := decoder.Decode(&zones); err != nil {
		return nil, err
	}
	return zones, nil
}
