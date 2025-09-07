package cmd

import (
	"context"
	"fmt"
	"reflect"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"

	"github.com/spf13/cobra"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var cpuLimit string
var memoryLimit string
var maxPods int

var createCmd = &cobra.Command{
	Use:   "create [username]",
	Short: "Create developer environment",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		username := args[0]
		namespace := "dev-" + username

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}

		ctx := context.Background()

		// Creating Namespace (Idempotent)
		_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {

			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Labels: map[string]string{
						"podcraft.dev/owner":   username,
						"podcraft.dev/managed": "true",
					},
				},
			}

			_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				panic(err)
			}

			fmt.Println("Namespace created:", namespace)
		} else {
			fmt.Println("Namespace already exists:", namespace)
		}

		// Creating ServiceAccount
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      username,
				Namespace: namespace,
			},
		}

		existingSA, err := clientset.CoreV1().
			ServiceAccounts(namespace).
			Get(ctx, username, metav1.GetOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = clientset.CoreV1().
					ServiceAccounts(namespace).
					Create(ctx, sa, metav1.CreateOptions{})
				if err != nil {
					panic(err)
				}
				fmt.Println("ServiceAccount created")
			} else {
				panic(err)
			}
		} else {
			fmt.Println("ServiceAccount already exists:", existingSA.Name)
		}

		// Creating Role
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      username + "-role",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"", "apps"},
					Resources: []string{"pods", "services", "deployments", "persistentvolumeclaims"},
					Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
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
					panic(err)
				}
				fmt.Println("Role created")
			} else {
				panic(err)
			}
		} else {

			// Compare rules
			if !reflect.DeepEqual(existingRole.Rules, role.Rules) {

				existingRole.Rules = role.Rules

				_, err = clientset.RbacV1().
					Roles(namespace).
					Update(ctx, existingRole, metav1.UpdateOptions{})

				if err != nil {
					panic(err)
				}

				fmt.Println("Role updated to match desired state")
			} else {
				fmt.Println("Role already matches desired state")
			}
		}

		// Creating RoleBinding
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
					panic(err)
				}
				fmt.Println("RoleBinding created")
			} else {
				panic(err)
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
					panic(err)
				}

				fmt.Println("RoleBinding updated")
			} else {
				fmt.Println("RoleBinding already matches desired state")
			}
		}

		// Creating Token for ServiceAccount
		tokenRequest := &authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				ExpirationSeconds: func() *int64 { t := int64(3600 * 24); return &t }(), // 24h
			},
		}

		tokenResponse, err := clientset.CoreV1().
			ServiceAccounts(namespace).
			CreateToken(ctx, username, tokenRequest, metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}

		token := tokenResponse.Status.Token
		fmt.Println("ServiceAccount token generated")

		// Load admin kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		adminConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		rawConfig, err := adminConfig.RawConfig()
		if err != nil {
			panic(err)
		}

		currentContext := rawConfig.CurrentContext
		clusterName := rawConfig.Contexts[currentContext].Cluster
		cluster := rawConfig.Clusters[clusterName]

		devConfig := api.Config{
			Clusters: map[string]*api.Cluster{
				"cluster": {
					Server:                   cluster.Server,
					CertificateAuthorityData: cluster.CertificateAuthorityData,
				},
			},
			Contexts: map[string]*api.Context{
				"dev-context": {
					Cluster:   "cluster",
					AuthInfo:  "dev-user",
					Namespace: namespace,
				},
			},
			AuthInfos: map[string]*api.AuthInfo{
				"dev-user": {
					Token: token,
				},
			},
			CurrentContext: "dev-context",
		}

		fileName := username + ".kubeconfig"

		err = clientcmd.WriteToFile(devConfig, fileName)
		if err != nil {
			panic(err)
		}

		fmt.Println("Kubeconfig written to:", fileName)

		// Applying Network Policies

		// Default Deny Ingress
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

		existingDenyNP, err := clientset.NetworkingV1().
			NetworkPolicies(namespace).
			Get(ctx, "default-deny", metav1.GetOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = clientset.NetworkingV1().
					NetworkPolicies(namespace).
					Create(ctx, defaultDeny, metav1.CreateOptions{})
				if err != nil {
					panic(err)
				}
				fmt.Println("Default deny policy created")
			} else {
				panic(err)
			}
		} else {

			if !reflect.DeepEqual(existingDenyNP.Spec, defaultDeny.Spec) {

				existingDenyNP.Spec = defaultDeny.Spec

				_, err = clientset.NetworkingV1().
					NetworkPolicies(namespace).
					Update(ctx, existingDenyNP, metav1.UpdateOptions{})

				if err != nil {
					panic(err)
				}

				fmt.Println("Default deny policy updated")
			} else {
				fmt.Println("Default deny policy already matches desired state")
			}
		}

		// Allowing Intra-Namespace Traffic
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

		existingInternalNP, err := clientset.NetworkingV1().
			NetworkPolicies(namespace).
			Get(ctx, "allow-same-namespace", metav1.GetOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = clientset.NetworkingV1().
					NetworkPolicies(namespace).
					Create(ctx, allowInternal, metav1.CreateOptions{})
				if err != nil {
					panic(err)
				}
				fmt.Println("Intra Namespace Traffic Policy created")
			} else {
				panic(err)
			}
		} else {

			if !reflect.DeepEqual(existingInternalNP.Spec, allowInternal.Spec) {

				existingInternalNP.Spec = allowInternal.Spec

				_, err = clientset.NetworkingV1().
					NetworkPolicies(namespace).
					Update(ctx, existingInternalNP, metav1.UpdateOptions{})

				if err != nil {
					panic(err)
				}

				fmt.Println("Intra Namespace Traffic policy updated")
			} else {
				fmt.Println("Intra Namespace Traffic policy already matches desired state")
			}
		}

		// Allowing Traffic From shared-services Namespace
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

		existingAllowNP, err := clientset.NetworkingV1().
			NetworkPolicies(namespace).
			Get(ctx, "allow-shared-services", metav1.GetOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				_, err = clientset.NetworkingV1().
					NetworkPolicies(namespace).
					Create(ctx, allowShared, metav1.CreateOptions{})
				if err != nil {
					panic(err)
				}
				fmt.Println("Shared Namespace Traffic Policy created")
			} else {
				panic(err)
			}
		} else {

			if !reflect.DeepEqual(existingAllowNP.Spec, allowShared.Spec) {

				existingAllowNP.Spec = allowShared.Spec

				_, err = clientset.NetworkingV1().
					NetworkPolicies(namespace).
					Update(ctx, existingAllowNP, metav1.UpdateOptions{})

				if err != nil {
					panic(err)
				}

				fmt.Println("Shared Namespace Traffic policy updated")
			} else {
				fmt.Println("Shared Namespace Traffic policy already matches desired state")
			}
		}

		fmt.Println("NetworkPolicies applied")

		// Applying ResourceQuota (Hardcoded Defaults)

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
					panic(err)
				}

				fmt.Println("ResourceQuota created")

			} else {
				panic(err)
			}
		} else {

			// Compare existing values
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
					panic(err)
				}

				fmt.Println("ResourceQuota updated to match desired state")

			} else {
				fmt.Println("ResourceQuota already matches desired state")
			}
		}

		// Applying LimitRange

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
					panic(err)
				}
				fmt.Println("LimitRange created")
			} else {
				panic(err)
			}
		} else {

			if !reflect.DeepEqual(existingLR.Spec, limitRange.Spec) {

				existingLR.Spec = limitRange.Spec

				_, err = clientset.CoreV1().
					LimitRanges(namespace).
					Update(ctx, existingLR, metav1.UpdateOptions{})

				if err != nil {
					panic(err)
				}

				fmt.Println("LimitRange updated")
			} else {
				fmt.Println("LimitRange already matches desired state")
			}
		}

		fmt.Println("Developer environment ready:", namespace)
		fmt.Println("\nStorage Policy:")
		fmt.Println("- Pods use ephemeral storage by default.")
		fmt.Println("- To persist data, create a PersistentVolumeClaim (PVC).")
		fmt.Println("- Maximum storage allowed in this namespace: 5Gi.")
		fmt.Println("- Deleting the namespace deletes all PVCs and data.")
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&cpuLimit, "cpu", "2", "Total CPU limit for namespace")
	createCmd.Flags().StringVar(&memoryLimit, "memory", "2Gi", "Total memory limit for namespace")
	createCmd.Flags().IntVar(&maxPods, "max-pods", 10, "Maximum number of pods")
}
