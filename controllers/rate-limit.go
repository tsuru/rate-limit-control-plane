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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/tsuru/rate-limit-control-plane/internal/config"
	ratelimit "github.com/tsuru/rate-limit-control-plane/pkg/rate-limit"
	rpaasOperatorv1alpha1 "github.com/tsuru/rpaas-operator/api/v1alpha1"
	"github.com/vmihailenco/msgpack/v5"
	"golang.org/x/sync/errgroup"
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
	fmt.Println("Reconciles")
	fmt.Println(config.Spec.ControllerMinutesInternval)
	ticker := time.NewTicker(config.Spec.ControllerMinutesInternval)
	for now := range ticker.C {
		fmt.Println(now)
		ctx := context.Background()

		rpaasList, err := c.listRpaasInstances(ctx)
		if err != nil {
			c.Log.Error(err, "Failed to list nginx pods")
			return
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
	fmt.Println("Reconciling Rpaas Instance")
	nginxList, err := c.getNginxInstancesFromRpaasInstance(ctx, rpaasInstance)
	if err != nil {
		return err
	}
	fmt.Println("Nginx list", len(nginxList.Items))

	if len(nginxList.Items) == 0 {
		c.Log.Info("No nginx instance was found", "rpaasInstance", rpaasInstance.Name)
		return nil
	}
	c.reconcileNginxRateLimits(ctx, rpaasInstance, nginxList.Items)
	return nil
}

func (c *RateLimitController) reconcileNginxRateLimits(ctx context.Context, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance, ngxInstances []corev1.Pod) error {
	rateLimitZones, err := c.getNginxRateLimitingZones(&ngxInstances[0])
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Printf("---> rateLimitZones: %s\n", strings.Join(rateLimitZones, ", "))

	for _, rateLimitZone := range rateLimitZones {
		if err := c.reconcileNginxRateLimitsZone(ctx, rateLimitZone, rpaasInstance, ngxInstances); err != nil {
			fmt.Printf("---> failed to reconcile zone %s: %s\n", rateLimitZone, err.Error())
		}
	}
	return nil
}

func (c *RateLimitController) getNginxRateLimitingZones(nginxInstance *corev1.Pod) ([]string, error) {
	zones := []string{}
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
	decoder := msgpack.NewDecoder(response.Body)
	if err := decoder.Decode(&zones); err != nil {
		return nil, err
	}
	return zones, nil
}

func (c *RateLimitController) reconcileNginxRateLimitsZone(ctx context.Context, zone string, rpaasInstance *rpaasOperatorv1alpha1.RpaasInstance, ngxInstances []corev1.Pod) error {
	rateLimitZoneEntriesByPod, err := c.getNginxRateLimitZoneEntriesByPod(zone, ngxInstances)
	if err != nil {
		return err
	}
	fmt.Printf("---> RATE LIMIT ENTRIES BY POD MAP FOR ZONE %s: %v\n", zone, rateLimitZoneEntriesByPod)
	c.aggregatRateLimitZoneEntries(rateLimitZoneEntriesByPod)
	return nil
}

func (c *RateLimitController) getNginxRateLimitZoneEntriesByPod(zone string, ngxInstances []corev1.Pod) (map[string][]ratelimit.RateLimitEntry, error) {
	rateLimitZoneEntriesByPod := make(map[string][]ratelimit.RateLimitEntry)
	resultChan := make(chan ratelimit.RateLimitPodZone)
	g := &errgroup.Group{}
	for _, nginxInstance := range ngxInstances {
		nginxInstance := nginxInstance
		fmt.Printf("---> %s\n", nginxInstance.Name)
		rateLimitZoneEntriesByPod[nginxInstance.Status.PodIP] = []ratelimit.RateLimitEntry{}
		g.Go(func() error {
			return c.getNginxRateLimitZoneEntries(&nginxInstance, zone, resultChan)
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	for range ngxInstances {
		rateLimitPodZone := <-resultChan
		rateLimitZoneEntriesByPod[rateLimitPodZone.PodIP] = rateLimitPodZone.RateLimitEntries
	}
	close(resultChan)
	return rateLimitZoneEntriesByPod, nil
}

// IP do POD: [IP do Client, last, excess]
// 10.144.0.1: [IPA, last, excess], [IPB, last, excess]
// 10.144.0.2: [IPA, last, excess], [IPB, last, excess]

func (c *RateLimitController) aggregatRateLimitZoneEntries(rateLimitZoneEntriesByPod map[string][]ratelimit.RateLimitEntry) {
	for podIP, rateLimitZoneEntries := range rateLimitZoneEntriesByPod {
		fmt.Printf("---> %s\n", podIP)
		for _, zoneEntry := range rateLimitZoneEntries {
			lastT := time.UnixMilli(zoneEntry.Last)
			fmt.Printf("------> Key: %s - Excess: %d - Last: %d - LastT: %v\n", string(zoneEntry.Key), zoneEntry.Excess, zoneEntry.Last, lastT)
		}
	}
}

func (c *RateLimitController) getNginxRateLimitZoneEntries(nginxInstance *corev1.Pod, zone string, resultChan chan ratelimit.RateLimitPodZone) error {
	rateLimitEntries := []ratelimit.RateLimitEntry{}
	endpoint := fmt.Sprintf("http://%s:%d/%s/%s", nginxInstance.Status.PodIP, administrativePort, "rate-limit", zone)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	decoder := msgpack.NewDecoder(response.Body)
	for {
		var message ratelimit.RateLimitEntry
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		rateLimitEntries = append(rateLimitEntries, message)
	}
	podZone := ratelimit.RateLimitPodZone{
		PodIP:            nginxInstance.Status.PodIP,
		Zone:             zone,
		RateLimitEntries: rateLimitEntries,
	}
	go func() {
		resultChan <- podZone
	}()
	return nil
}

// -> Ler Rpaas (Rate-Limit?)
// -> Ler pods de NGINXs
// -> Para cada pod:
//   -> (goroutine) Ler rate limits
// -> juntar resultados
