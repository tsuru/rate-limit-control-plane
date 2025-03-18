package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RateLimitController struct {
	client.Client
	Log logr.Logger
}

func (c *RateLimitController) Reconcile() {
	fmt.Println("Reconcile")
	ticker := time.NewTicker(time.Second * 10)
	for now := range ticker.C {
		fmt.Println(now)
		ctx := context.Background()

		rpaasList := rpaasOperatorv1alpha1.RpaasInstanceList{}
		if err := c.Client.List(ctx, &rpaasList, &client.ListOptions{
			// LabelSelector:         nil,
			Namespace: "rpaasv2-be-gke-main",
		}); err != nil {
			c.Log.Error(err, "Failed to list nginx pods")
		}
		fmt.Println(rpaasList)
		ticker.Stop()
	}
	// TODO: Ticker de 5 mins
}

// -> Ler Rpaas (Rate-Limit?)
// -> Ler pods de NGINXs
// -> Para cada pod:
//   -> (goroutine) Ler rate limits
// -> juntar resultados
