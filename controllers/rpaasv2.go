package controllers

import (
	"fmt"
	"strings"
)

// instanceNameFromDeploymentPodName extracts the RPAAS instance name from a pod name that follows the deployment naming convention.
// the deployment naming convention is typically "<instance-name>-<deployment-name>-<random-suffix>"
// the returned instance name is the part before the first hyphen in the pod name.
func instanceNameFromDeploymentPodName(podName string) (string, error) {
	parts := strings.Split(podName, "-")
	if len(parts) < 3 {
		return "", fmt.Errorf("pod name does not match expected pattern (<instance-name>-<deployment-name>-<random-suffix>): %s", podName)
	}
	instanceName := strings.Join(parts[:len(parts)-2], "-")
	return instanceName, nil
}
