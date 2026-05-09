package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset *kubernetes.Clientset
}

func NewClient(kubeconfigPath string) (*Client, error) {
	var cfg *rest.Config
	var err error
	if kubeconfigPath != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: cs}, nil
}

func (c *Client) PatchMemoryLimit(namespace, podName, newLimit string) error {
	ctx := context.Background()

	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get pod: %w", err)
	}

	deployName, ok := pod.Labels["app"]
	if !ok {
		return fmt.Errorf("pod has no 'app' label")
	}

	containerName := ""
	for i := range pod.Spec.Containers {
		n := pod.Spec.Containers[i].Name
		if n == "nginx" {
			containerName = "nginx"
			break
		}
	}
	if containerName == "" {
		for i := range pod.Spec.Containers {
			n := pod.Spec.Containers[i].Name
			if n != "stress" {
				containerName = n
				break
			}
		}
	}
	if containerName == "" {
		return fmt.Errorf("could not determine container name to patch")
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name": containerName,
							"resources": map[string]interface{}{
								"limits": map[string]string{"memory": newLimit},
							},
						},
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	// Strategic merge merges pod.spec.containers by "name". JSON merge patch would replace
	// the whole containers array and drop required fields (e.g. image) → validation error.
	_, err = c.clientset.AppsV1().Deployments(namespace).Patch(
		ctx, deployName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}
