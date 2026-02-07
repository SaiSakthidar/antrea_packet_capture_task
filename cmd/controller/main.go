package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/packet-capture-controller/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Fatal("NODE_NAME environment variable must be set")
	}
	klog.Infof("Starting packet capture controller on node: %s", nodeName)

	config, err := getKubeConfig()
	if err != nil {
		klog.Fatalf("Failed to get Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		klog.Infof("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()

	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	klog.Infof("Creating informer with field selector: %s", fieldSelector)

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		30*time.Second,
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fieldSelector
		}),
	)

	ctrl := controller.NewController(clientset, informerFactory, nodeName)

	klog.Info("Starting informer factory")
	informerFactory.Start(ctx.Done())

	klog.Info("Packet capture controller started successfully")

	if err := ctrl.Run(ctx.Done()); err != nil {
		klog.Fatalf("Error running controller: %v", err)
	}

	<-ctx.Done()
	klog.Info("Shutting down packet capture controller")

	time.Sleep(2 * time.Second)
	klog.Info("Packet capture controller stopped")
}

func getKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		klog.Info("Using in-cluster Kubernetes config")
		return config, nil
	}

	klog.Infof("In-cluster config not available: %v", err)
	klog.Info("Attempting to use out-of-cluster config from kubeconfig")

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = fmt.Sprintf("%s/.kube/config", homeDir)
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	klog.Infof("Using out-of-cluster config from: %s", kubeconfig)
	return config, nil
}
