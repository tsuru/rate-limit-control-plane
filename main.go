// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	nginxOperatorv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"github.com/tsuru/rate-limit-control-plane/controllers"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"k8s.io/api/node/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kedav1alpha1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	_ = rpaasOperatorv1alpha1.AddToScheme(scheme)
	_ = nginxOperatorv1alpha1.AddToScheme(scheme)
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// MetricsBindAddress: *metricsAddr,
		// Port: 9443,
		// LeaderElection:     *enableLeaderElection,
		// LeaderElectionID:   "1803asdt.tsuru.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	manager := manager.NewGoroutine()
	if err = (&controllers.RateLimitControllerReconcile{
		Client:           mgr.GetClient(),
		Log:              mgr.GetLogger().WithName("controllers").WithName("RateLimitControllerReconcile"),
		ManagerGoroutine: manager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RateLimitControllerReconcile")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
