package network

import (
	"context"
	"fmt"
	"reflect"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func EnsureNetwork(clientset *kubernetes.Clientset, namespace string) error {

	ctx := context.Background()

	// -------------------------
	// 1. Default Deny
	// -------------------------

	defaultDeny := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-deny",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}

	existingDeny, err := clientset.NetworkingV1().
		NetworkPolicies(namespace).
		Get(ctx, "default-deny", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Create(ctx, defaultDeny, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Default deny policy created")
		} else {
			return err
		}
	} else {
		if !reflect.DeepEqual(existingDeny.Spec, defaultDeny.Spec) {
			existingDeny.Spec = defaultDeny.Spec
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Update(ctx, existingDeny, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Default deny policy updated")
		} else {
			fmt.Println("Default deny policy already matches desired state")
		}
	}

	// -------------------------
	// 2. Allow Same Namespace
	// -------------------------

	allowInternal := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-same-namespace",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}

	existingInternal, err := clientset.NetworkingV1().
		NetworkPolicies(namespace).
		Get(ctx, "allow-same-namespace", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Create(ctx, allowInternal, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Intra-namespace policy created")
		} else {
			return err
		}
	} else {
		if !reflect.DeepEqual(existingInternal.Spec, allowInternal.Spec) {
			existingInternal.Spec = allowInternal.Spec
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Update(ctx, existingInternal, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Intra-namespace policy updated")
		} else {
			fmt.Println("Intra-namespace policy already matches desired state")
		}
	}

	// -------------------------
	// 3. Allow Shared Services
	// -------------------------

	allowShared := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-shared-services",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"podcraft.dev/shared": "true",
								},
							},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}

	existingShared, err := clientset.NetworkingV1().
		NetworkPolicies(namespace).
		Get(ctx, "allow-shared-services", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Create(ctx, allowShared, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Shared namespace policy created")
		} else {
			return err
		}
	} else {
		if !reflect.DeepEqual(existingShared.Spec, allowShared.Spec) {
			existingShared.Spec = allowShared.Spec
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Update(ctx, existingShared, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Shared namespace policy updated")
		} else {
			fmt.Println("Shared namespace policy already matches desired state")
		}
	}

	fmt.Println("NetworkPolicies ensured")

	return nil
}
