package rbac

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func EnsureRBAC(clientset *kubernetes.Clientset, namespace, username string) error {

	ctx := context.Background()

	// ServiceAccount
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username,
			Namespace: namespace,
		},
	}

	_, err := clientset.CoreV1().
		ServiceAccounts(namespace).
		Get(ctx, username, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.CoreV1().
				ServiceAccounts(namespace).
				Create(ctx, sa, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("ServiceAccount created")
		} else {
			return err
		}
	} else {
		fmt.Println("ServiceAccount already exists:", username)
	}

	// Role
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username + "-role",
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "apps"},
				Resources: []string{
					"pods",
					"services",
					"deployments",
					"persistentvolumeclaims",
				},
				Verbs: []string{"get", "list", "watch", "create", "delete", "update"},
			},
		},
	}

	existingRole, err := clientset.RbacV1().
		Roles(namespace).
		Get(ctx, username+"-role", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.RbacV1().
				Roles(namespace).
				Create(ctx, role, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Role created")
		} else {
			return err
		}
	} else {

		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			_, err = clientset.RbacV1().
				Roles(namespace).
				Update(ctx, existingRole, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("Role updated")
		} else {
			fmt.Println("Role already matches desired state")
		}
	}

	// RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      username + "-binding",
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      username,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     username + "-role",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	existingRB, err := clientset.RbacV1().
		RoleBindings(namespace).
		Get(ctx, username+"-binding", metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = clientset.RbacV1().
				RoleBindings(namespace).
				Create(ctx, roleBinding, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			fmt.Println("RoleBinding created")
		} else {
			return err
		}
	} else {

		if !reflect.DeepEqual(existingRB.Subjects, roleBinding.Subjects) ||
			!reflect.DeepEqual(existingRB.RoleRef, roleBinding.RoleRef) {

			existingRB.Subjects = roleBinding.Subjects
			existingRB.RoleRef = roleBinding.RoleRef

			_, err = clientset.RbacV1().
				RoleBindings(namespace).
				Update(ctx, existingRB, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			fmt.Println("RoleBinding updated")
		} else {
			fmt.Println("RoleBinding already matches desired state")
		}
	}

	return nil
}
