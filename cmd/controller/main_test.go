package main

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

func TestNodeLocalPodFiltering(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("field selector filters pods by node name", prop.ForAll(
		func(targetNode string, pods []testPod) bool {
			fieldSelector := fields.OneTermEqualSelector("spec.nodeName", targetNode)

			expectedPods := []testPod{}
			for _, pod := range pods {
				if pod.nodeName == targetNode {
					expectedPods = append(expectedPods, pod)
				}
			}

			actualPods := []testPod{}
			for _, pod := range pods {
				k8sPod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod.name,
						Namespace: pod.namespace,
					},
					Spec: corev1.PodSpec{
						NodeName: pod.nodeName,
					},
				}
				if matchesFieldSelector(fieldSelector, k8sPod) {
					actualPods = append(actualPods, pod)
				}
			}

			if len(actualPods) != len(expectedPods) {
				t.Logf("Length mismatch: expected %d, got %d", len(expectedPods), len(actualPods))
				return false
			}

			for _, pod := range actualPods {
				if pod.nodeName != targetNode {
					t.Logf("Pod %s/%s on node %s should not match target node %s",
						pod.namespace, pod.name, pod.nodeName, targetNode)
					return false
				}
			}

			return true
		},
		genNodeName(),
		genPodList(),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

type testPod struct {
	name      string
	namespace string
	nodeName  string
}

func genNodeName() gopter.Gen {
	return gen.OneConstOf("node-1", "node-2", "node-3", "node-4", "node-5")
}

func genPodList() gopter.Gen {
	return gen.SliceOf(genPod())
}

func genPod() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),                                                    // name
		gen.OneConstOf("default", "kube-system", "test-ns", "production"), // namespace
		genNodeName(), // nodeName
	).Map(func(values []interface{}) testPod {
		return testPod{
			name:      values[0].(string),
			namespace: values[1].(string),
			nodeName:  values[2].(string),
		}
	})
}

func matchesFieldSelector(selector fields.Selector, pod *corev1.Pod) bool {
	fieldSet := fields.Set{
		"metadata.name":      pod.Name,
		"metadata.namespace": pod.Namespace,
		"spec.nodeName":      pod.Spec.NodeName,
	}

	return selector.Matches(fieldSet)
}
