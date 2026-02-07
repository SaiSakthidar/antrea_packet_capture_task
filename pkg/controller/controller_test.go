package controller

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAnnotationChangeDetection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("adding capture annotation enqueues pod", prop.ForAll(
		func(podName, namespace, nodeName, annotationValue string) bool {
			clientset := fake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
			ctrl := NewController(clientset, informerFactory, nodeName)

			oldPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            podName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Annotations:     map[string]string{},
				},
				Spec: corev1.PodSpec{
					NodeName: nodeName,
				},
			}

			newPod := oldPod.DeepCopy()
			newPod.ResourceVersion = "2"
			newPod.Annotations[CaptureAnnotation] = annotationValue

			for ctrl.queue.Len() > 0 {
				item, _ := ctrl.queue.Get()
				ctrl.queue.Done(item)
			}

			ctrl.handlePodUpdate(oldPod, newPod)

			return ctrl.queue.Len() == 1
		},
		gen.Identifier(),
		gen.OneConstOf("default", "kube-system", "test-ns"),
		gen.OneConstOf("node-1", "node-2"),
		gen.IntRange(1, 10).Map(func(n int) string { return string(rune('0' + n)) }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestAnnotationRemovalDetection(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("removing capture annotation enqueues pod", prop.ForAll(
		func(podName, namespace, nodeName, annotationValue string) bool {
			clientset := fake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(clientset, 30*time.Second)
			ctrl := NewController(clientset, informerFactory, nodeName)

			oldPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            podName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Annotations: map[string]string{
						CaptureAnnotation: annotationValue,
					},
				},
				Spec: corev1.PodSpec{
					NodeName: nodeName,
				},
			}

			newPod := oldPod.DeepCopy()
			newPod.ResourceVersion = "2"
			delete(newPod.Annotations, CaptureAnnotation)

			for ctrl.queue.Len() > 0 {
				item, _ := ctrl.queue.Get()
				ctrl.queue.Done(item)
			}

			ctrl.handlePodUpdate(oldPod, newPod)

			return ctrl.queue.Len() == 1
		},
		gen.Identifier(),
		gen.OneConstOf("default", "kube-system", "test-ns"),
		gen.OneConstOf("node-1", "node-2"),
		gen.IntRange(1, 10).Map(func(n int) string { return string(rune('0' + n)) }),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

