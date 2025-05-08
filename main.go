// Copyright 2025 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"os"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	nginxOperatorv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	"github.com/tsuru/rate-limit-control-plane/controllers"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
	"github.com/tsuru/rate-limit-control-plane/internal/repository"
	"github.com/tsuru/rate-limit-control-plane/server"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"k8s.io/api/node/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

type configOpts struct {
	metricsAddr                string
	healthAddr                 string
	internalAPIAddr            string
	leaderElection             bool
	leaderElectionResourceName string
}

func (o *configOpts) bindFlags(fs *flag.FlagSet) {
	// Following the standard of flags on Kubernetes.
	// See more: https://github.com/kubernetes-sigs/kubebuilder/issues/1839
	fs.StringVar(&o.metricsAddr, "metrics-bind-address", ":8080", "The TCP address that controller should bind to for serving Prometheus metrics. It can be set to \"0\" to disable the metrics serving.")
	fs.StringVar(&o.healthAddr, "health-probe-bind-address", ":8081", "The TCP address that controller should bind to for serving health probes.")
	fs.StringVar(&o.internalAPIAddr, "internal-api-address", ":8082", "The TCP address that controller should bind to for internal controller API.")

	fs.BoolVar(&o.leaderElection, "leader-elect", false, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	fs.StringVar(&o.leaderElectionResourceName, "leader-elect-resource-name", "rate-limit-control-plane-lock", "The name of resource object that is used for locking during leader election.")
}

type InternalAPIServer struct {
	repo         *repository.ZoneDataRepository
	internalAddr string
}

func (s *InternalAPIServer) Start(ctx context.Context) error {
	setupLog.Info("leadership acquired, starting internalapi", "addr", s.internalAddr)
	go func() {
		server.Notification(s.repo, s.internalAddr)
	}()
	<-ctx.Done()
	return nil
}

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kedav1alpha1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	_ = rpaasOperatorv1alpha1.AddToScheme(scheme)
	_ = nginxOperatorv1alpha1.AddToScheme(scheme)
}

func main() {
	var opts configOpts
	opts.bindFlags(flag.CommandLine)

	zapOpts := zap.Options{}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	repo, ch := repository.NewRpaasZoneDataRepository()
	go repo.StartReader()

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		setupLog.Info("NAMESPACE not set, watching all namespaces")
	} else {
		setupLog.Info("Running in namespace", "namespace", namespace)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		Namespace:                  namespace,
		MetricsBindAddress:         opts.metricsAddr,
		LeaderElectionResourceLock: "leases",
		LeaderElection:             opts.leaderElection,
		LeaderElectionID:           opts.leaderElectionResourceName,
		LeaderElectionNamespace:    namespace,
		HealthProbeBindAddress:     opts.healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	mgr.AddMetricsExtraHandler("/pod-worker-read-latency", promhttp.Handler())

	internalAPIServer := &InternalAPIServer{
		repo:         repo,
		internalAddr: opts.internalAPIAddr,
	}

	err = mgr.Add(internalAPIServer)
	if err != nil {
		setupLog.Error(err, "unable to add internal API server")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	manager := manager.NewGoroutineManager()
	if err = (&controllers.RateLimitControllerReconcile{
		Client:           mgr.GetClient(),
		Log:              mgr.GetLogger().WithName("controllers").WithName("RateLimitControllerReconcile"),
		ManagerGoroutine: manager,
		Notify:           ch,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RateLimitControllerReconcile")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
