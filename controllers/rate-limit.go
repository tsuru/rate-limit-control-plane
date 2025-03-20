package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/tsuru/rate-limit-control-plane/internal/config"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	flavor             = "global-ratelimit"
	administrativePort = 8800
)

type RateLimitController struct {
	client.Client
	Log logr.Logger
}

func (c *RateLimitController) Reconcile() {
	fmt.Println("Reconcile")
	fmt.Println(config.Spec.ControllerMinutesInternval)
	ticker := time.NewTicker(config.Spec.ControllerMinutesInternval)
	for now := range ticker.C {
		fmt.Println(now)
		ctx := context.Background()

		rpaasList, err := c.listRpaasInstances(ctx)
		if err != nil {
			c.Log.Error(err, "Failed to list nginx pods")
		}
		for _, rpaasInstance := range rpaasList.Items {
			if slices.Contains(rpaasInstance.Spec.Flavors, flavor) {
				c.reconcileRpaasInstance(ctx, &rpaasInstance)
			}
		}
	}
}

func (c *RateLimitController) listRpaasInstances(ctx context.Context) (rpaasOperatorv1alpha1.RpaasInstanceList, error) {
	rpaasList := rpaasOperatorv1alpha1.RpaasInstanceList{}
	err := c.List(ctx, &rpaasList, &client.ListOptions{
		Namespace: "rpaasv2",
	})
	return rpaasList, err
}

func (c *RateLimitController) getNginxInstancesFromRpaasInstance(ctx context.Context, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance) (corev1.PodList, error) {
	nginxPodList := corev1.PodList{}
	err := c.List(ctx, &nginxPodList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"rpaas.extensions.tsuru.io/instance-name": rpaasInstance.Name,
		}),
		Namespace: "rpaasv2",
	})
	return nginxPodList, err
}

func (c *RateLimitController) reconcileRpaasInstance(ctx context.Context, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance) error {
	nginxList, err := c.getNginxInstancesFromRpaasInstance(ctx, rpaasInstance)
	if err != nil {
		return err
	}

	if len(nginxList.Items) == 0 {
		return nil
	}
	c.reconcileNginxRateLimits(ctx, rpaasInstance, nginxList.Items)
	return nil
}

func (c *RateLimitController) reconcileNginxRateLimits(ctx context.Context, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance, ngxInstances []corev1.Pod) error {
	fmt.Println(rpaasInstance.Name, rpaasInstance.Spec.Flavors)
	rateLimitZones, err := c.getNginxRateLimitingZones(&ngxInstances[0])
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Printf("---> %s\n", strings.Join(rateLimitZones, ", "))

	for _, nginxInstance := range ngxInstances {
		fmt.Printf("---> %s\n", nginxInstance.Name)
	}
	return nil
}

func (c *RateLimitController) getNginxRateLimitingZones(nginxInstance *corev1.Pod) ([]string, error) {
	fmt.Println(nginxInstance.Name, nginxInstance.Status.PodIP)
	endpoint := fmt.Sprintf("http://%s:%d/%s", nginxInstance.Status.PodIP, administrativePort, "rate-limit")
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	// TODO: Get mmessagepack
	fmt.Printf("---> responseBody: %s - status code: %d\n", string(responseBody), response.StatusCode)
	return nil, nil
}

// -> Ler Rpaas (Rate-Limit?)
// -> Ler pods de NGINXs
// -> Para cada pod:
//   -> (goroutine) Ler rate limits
// -> juntar resultados
