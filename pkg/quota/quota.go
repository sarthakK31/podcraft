package quota

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func EnsureQuota(clientset *kubernetes.Clientset, namespace string, cpuLimit string, memoryLimit string, maxPods int) error {

	ctx := context.Background()

	// -------------------------
	// ResourceQuota
	// -------------------------

	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev-quota",
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourcePods:            resource.MustParse(fmt.Sprintf("%d", maxPods)),
				corev1.ResourceLimitsCPU:       resource.MustParse(cpuLimit),
				corev1.ResourceLimitsMemory:    resource.MustParse(memoryLimit),
				corev1.ResourceRequestsStorage: resource.MustParse("5Gi"),
			},
		},
	}

	existingQuota, err := clientset.CoreV1().
		ResourceQuotas(namespace).
		Get(ctx, "dev-quota", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.CoreV1().
				ResourceQuotas(namespace).
				Create(ctx, quota, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("ResourceQuota created")
		} else {
			return err
		}
	} else {

		desiredCPU := resource.MustParse(cpuLimit)
		desiredMemory := resource.MustParse(memoryLimit)
		desiredPods := resource.MustParse(fmt.Sprintf("%d", maxPods))
		desiredStorage := resource.MustParse("5Gi")

		currentCPU := existingQuota.Spec.Hard[corev1.ResourceLimitsCPU]
		currentMemory := existingQuota.Spec.Hard[corev1.ResourceLimitsMemory]
		currentPods := existingQuota.Spec.Hard[corev1.ResourcePods]
		currentStorage := existingQuota.Spec.Hard[corev1.ResourceRequestsStorage]

		if currentCPU.Cmp(desiredCPU) != 0 ||
			currentMemory.Cmp(desiredMemory) != 0 ||
			currentPods.Cmp(desiredPods) != 0 ||
			currentStorage.Cmp(desiredStorage) != 0 {

			existingQuota.Spec.Hard[corev1.ResourceLimitsCPU] = desiredCPU
			existingQuota.Spec.Hard[corev1.ResourceLimitsMemory] = desiredMemory
			existingQuota.Spec.Hard[corev1.ResourcePods] = desiredPods
			existingQuota.Spec.Hard[corev1.ResourceRequestsStorage] = desiredStorage

			_, err = clientset.CoreV1().
				ResourceQuotas(namespace).
				Update(ctx, existingQuota, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			fmt.Println("ResourceQuota updated to match desired state")
		} else {
			fmt.Println("ResourceQuota already matches desired state")
		}
	}

	// -------------------------
	// LimitRange
	// -------------------------

	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev-limitrange",
			Namespace: namespace,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}

	existingLR, err := clientset.CoreV1().
		LimitRanges(namespace).
		Get(ctx, "dev-limitrange", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.CoreV1().
				LimitRanges(namespace).
				Create(ctx, limitRange, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("LimitRange created")
		} else {
			return err
		}
	} else {

		if !reflect.DeepEqual(existingLR.Spec, limitRange.Spec) {

			existingLR.Spec = limitRange.Spec

			_, err = clientset.CoreV1().
				LimitRanges(namespace).
				Update(ctx, existingLR, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			fmt.Println("LimitRange updated")
		} else {
			fmt.Println("LimitRange already matches desired state")
		}
	}

	return nil
}
