package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	nginxOperatorv1alpha1 "github.com/tsuru/nginx-operator/api/v1alpha1"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
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

		rpaasList, err := c.listRpaasInstances(ctx)
		if err != nil {
			c.Log.Error(err, "Failed to list nginx pods")
		}
		for _, rpaasInstance := range rpaasList.Items {
			c.reconcileRpaasInstance(ctx, &rpaasInstance)
		}
	}
	// TODO: Ticker de 5 mins
}

func (c *RateLimitController) listRpaasInstances(ctx context.Context) (rpaasOperatorv1alpha1.RpaasInstanceList, error) {
	rpaasList := rpaasOperatorv1alpha1.RpaasInstanceList{}
	err := c.List(ctx, &rpaasList, &client.ListOptions{
		// LabelSelector:         nil,
		Namespace: "rpaasv2-be-gke-main",
	})
	return rpaasList, err
}

func (c *RateLimitController) getNginxInstancesFromRpaasInstance(ctx context.Context, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance) (nginxOperatorv1alpha1.NginxList, error) {
	nginxList := nginxOperatorv1alpha1.NginxList{}
	err := c.List(ctx, &nginxList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"rpaas.extensions.tsuru.io/instance-name": rpaasInstance.Name,
		}),
		Namespace: "rpaasv2-be-gke-main",
	})
	return nginxList, err
}

func (c *RateLimitController) reconcileRpaasInstance(ctx context.Context, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance) error {
	fmt.Println(rpaasInstance.Name)
	nginxList, err := c.getNginxInstancesFromRpaasInstance(ctx, rpaasInstance)
	if err != nil {
		return err
	}

	for _, nginxInstance := range nginxList.Items {
		fmt.Printf("---> %s\n", nginxInstance.Name)
	}
	return nil
}

// -> Ler Rpaas (Rate-Limit?)
// -> Ler pods de NGINXs
// -> Para cada pod:
//   -> (goroutine) Ler rate limits
// -> juntar resultados
