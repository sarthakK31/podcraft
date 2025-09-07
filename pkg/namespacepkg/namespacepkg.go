package namespacepkg

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func EnsureNamespace(clientset *kubernetes.Clientset, namespaceName, owner string) error {

	ctx := context.Background()

	_, err := clientset.CoreV1().
		Namespaces().
		Get(ctx, namespaceName, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
					Labels: map[string]string{
						"podcraft.dev/owner":   owner,
						"podcraft.dev/managed": "true",
					},
				},
			}

			_, err = clientset.CoreV1().
				Namespaces().
				Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			fmt.Println("Namespace created:", namespaceName)
		} else {
			return err
		}
	} else {
		fmt.Println("Namespace already exists:", namespaceName)
	}

	return nil
}
