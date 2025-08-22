package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

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

		_, _ = clientset.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
		fmt.Println("RoleBinding ensured")

		fmt.Println("Developer environment ready:", namespace)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
}
