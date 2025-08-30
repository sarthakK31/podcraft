package cmd

import (
	"context"
	"fmt"

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

		_, _ = clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{})
		fmt.Println("ServiceAccount ensured")

		// Creating Role
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      username + "-role",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"", "apps"},
					Resources: []string{"pods", "services", "deployments"},
					Verbs:     []string{"get", "list", "watch", "create", "delete", "update"},
				},
			},
		}

		_, _ = clientset.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
		fmt.Println("Role ensured")

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

		existing, err := clientset.NetworkingV1().
			NetworkPolicies(namespace).
			Get(ctx, "default-deny", metav1.GetOptions{})

		if err != nil {
			// Create if not found
			_, err = clientset.NetworkingV1().
				NetworkPolicies(namespace).
				Create(ctx, defaultDeny, metav1.CreateOptions{})
			if err != nil {
				panic(err)
			}
			fmt.Println("Default deny policy created")
		} else {
			fmt.Println("Default deny already exists:", existing.Name)
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

		_, _ = clientset.NetworkingV1().
			NetworkPolicies(namespace).
			Create(ctx, allowInternal, metav1.CreateOptions{})

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

		_, _ = clientset.NetworkingV1().
			NetworkPolicies(namespace).
			Create(ctx, allowShared, metav1.CreateOptions{})

		fmt.Println("NetworkPolicies applied")

		// 7️⃣ Apply ResourceQuota (Hardcoded Defaults)

		quota := &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev-quota",
				Namespace: namespace,
			},
			Spec: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourcePods:         resource.MustParse(fmt.Sprintf("%d", maxPods)),
					corev1.ResourceLimitsCPU:    resource.MustParse(cpuLimit),
					corev1.ResourceLimitsMemory: resource.MustParse(memoryLimit),
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
			fmt.Println("ResourceQuota already exists:", existingQuota.Name)
		}

		// 8️⃣ Apply LimitRange

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

		_, err = clientset.CoreV1().
			LimitRanges(namespace).
			Create(ctx, limitRange, metav1.CreateOptions{})

		if err != nil {
			fmt.Println("LimitRange already exists or error:", err)
		} else {
			fmt.Println("LimitRange applied")
		}
		//**************************

		_, _ = clientset.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
		fmt.Println("RoleBinding ensured")

		fmt.Println("Developer environment ready:", namespace)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&cpuLimit, "cpu", "2", "Total CPU limit for namespace")
	createCmd.Flags().StringVar(&memoryLimit, "memory", "2Gi", "Total memory limit for namespace")
	createCmd.Flags().IntVar(&maxPods, "max-pods", 10, "Maximum number of pods")
}
