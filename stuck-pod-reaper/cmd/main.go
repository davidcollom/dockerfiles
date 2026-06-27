package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/davidcollom/dockerfiles/stuck-pod-reaper/internal/reaper"
)

func main() {
	var (
		kubeconfig = flag.String("kubeconfig", "", "Path to kubeconfig, optional in-cluster")
		threshold  = flag.Duration("threshold", 15*time.Minute, "Minimum pod age before action")
		deletePods = flag.Bool("delete", false, "Actually delete stuck pods")
		namespace  = flag.String("namespace", "", "Namespace to scan, empty means all namespaces")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := kubeConfig(*kubeconfig)
	if err != nil {
		logger.Error("failed to create kube config", "error", err)
		os.Exit(1)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	r := reaper.New(client, logger, reaper.Options{
		Threshold: *threshold,
		Delete:    *deletePods,
		Namespace: *namespace,
		Now:       time.Now,
	})

	if err := r.Run(context.Background()); err != nil {
		logger.Error("reaper failed", "error", err)
		os.Exit(1)
	}
}

func kubeConfig(path string) (*rest.Config, error) {
	if path != "" {
		return clientcmd.BuildConfigFromFlags("", path)
	}

	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}
