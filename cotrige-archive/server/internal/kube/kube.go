package kube

import (
	"fmt"

	"github.com/pangjian-pj/cotriage-dev/server/internal/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Kube struct {
	Client *kubernetes.Clientset
}

func New(cfg config.Config) (*Kube, error) {
	var rc *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		rc, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("build config from kubeconfig: %w", err)
		}
	} else {
		rc, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w (hint: set KUBECONFIG for local runs)", err)
		}
	}

	cs, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("new clientset: %w", err)
	}
	return &Kube{Client: cs}, nil
}
