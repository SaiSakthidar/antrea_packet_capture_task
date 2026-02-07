package controller

import (
	"fmt"
	"time"

	"github.com/packet-capture-controller/pkg/capture"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const CaptureAnnotation = "tcpdump.antrea.io"

type Controller struct {
	clientset      kubernetes.Interface
	podInformer    cache.SharedIndexInformer
	queue          workqueue.TypedRateLimitingInterface[string]
	nodeName       string
	workerCount    int
	captureManager *capture.Manager
}

func NewController(
	clientset kubernetes.Interface,
	informerFactory informers.SharedInformerFactory,
	nodeName string,
) *Controller {
	queue := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[string](),
	)

	podInformer := informerFactory.Core().V1().Pods().Informer()

	controller := &Controller{
		clientset:      clientset,
		podInformer:    podInformer,
		queue:          queue,
		nodeName:       nodeName,
		workerCount:    1,
		captureManager: capture.NewManager(),
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.handlePodAdd,
		UpdateFunc: controller.handlePodUpdate,
		DeleteFunc: controller.handlePodDelete,
	})

	return controller
}

func (c *Controller) handlePodAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("Failed to get key for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	if _, hasAnnotation := pod.Annotations[CaptureAnnotation]; hasAnnotation {
		klog.V(2).Infof("Pod added with capture annotation: %s", key)
		c.queue.Add(key)
	}
}

func (c *Controller) handlePodUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	if oldPod.ResourceVersion == newPod.ResourceVersion {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(newPod)
	if err != nil {
		klog.Errorf("Failed to get key for pod %s/%s: %v", newPod.Namespace, newPod.Name, err)
		return
	}

	oldValue, oldHasAnnotation := oldPod.Annotations[CaptureAnnotation]
	newValue, newHasAnnotation := newPod.Annotations[CaptureAnnotation]

	if (!oldHasAnnotation && newHasAnnotation) || (oldHasAnnotation && newHasAnnotation && oldValue != newValue) {
		klog.V(2).Infof("Pod annotation added/changed: %s", key)
		c.queue.Add(key)
	}

	if oldHasAnnotation && !newHasAnnotation {
		klog.V(2).Infof("Pod annotation removed: %s", key)
		c.queue.Add(key)
	}
}

func (c *Controller) handlePodDelete(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Error decoding object, invalid type")
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			klog.Errorf("Error decoding tombstone object, invalid type")
			return
		}
		klog.V(4).Infof("Recovered deleted pod %s/%s from tombstone", pod.Namespace, pod.Name)
	}

	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("Failed to get key for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return
	}

	if _, hasAnnotation := pod.Annotations[CaptureAnnotation]; hasAnnotation {
		klog.V(2).Infof("Pod deleted with capture annotation: %s", key)
		c.queue.Add(key)
	}
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting packet capture controller")

	if !cache.WaitForCacheSync(stopCh, c.podInformer.HasSynced) {
		return fmt.Errorf("failed to wait for cache sync")
	}

	klog.Info("Cache synced, starting workers")

	for i := 0; i < c.workerCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Infof("Started %d workers", c.workerCount)

	<-stopCh
	klog.Info("Shutting down packet capture controller")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	runtime.HandleError(fmt.Errorf("error syncing pod %q: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("invalid key: %s", key)
	}

	obj, exists, err := c.podInformer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get pod from cache: %w", err)
	}

	if !exists {
		klog.V(2).Infof("Pod %s no longer exists, cleaning up", key)
		c.captureManager.StopCapture(namespace, name)
		return nil
	}

	pod := obj.(*corev1.Pod)

	if pod.DeletionTimestamp != nil {
		klog.V(2).Infof("Pod %s is being deleted, stopping capture", key)
		c.captureManager.StopCapture(namespace, name)
		return nil
	}

	_, hasAnnotation := pod.Annotations[CaptureAnnotation]
	if hasAnnotation {
		klog.V(2).Infof("Starting capture for pod %s", key)
		if err := c.captureManager.StartCapture(pod); err != nil {
			return fmt.Errorf("failed to start capture for pod %s: %w", key, err)
		}
	} else {
		klog.V(2).Infof("Stopping capture for pod %s (annotation removed)", key)
		c.captureManager.StopCapture(namespace, name)
	}

	klog.V(4).Infof("Successfully synced pod %s/%s", namespace, name)
	return nil
}
