package kubeconfigpkg

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Generate(clientset *kubernetes.Clientset, kubeconfigPath string, namespace string, username string) error {

	ctx := context.Background()

	// -------------------------
	// 1. Generate Token
	// -------------------------

	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			ExpirationSeconds: func() *int64 {
				t := int64(3600 * 24) // 24 hours
				return &t
			}(),
		},
	}

	tokenResponse, err := clientset.CoreV1().
		ServiceAccounts(namespace).
		CreateToken(ctx, username, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	token := tokenResponse.Status.Token
	fmt.Println("ServiceAccount token generated")

	// -------------------------
	// 2. Load Admin Kubeconfig
	// -------------------------

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfigPath

	configOverrides := &clientcmd.ConfigOverrides{}
	adminConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	rawConfig, err := adminConfig.RawConfig()
	if err != nil {
		return err
	}

	currentContext := rawConfig.CurrentContext
	clusterName := rawConfig.Contexts[currentContext].Cluster
	cluster := rawConfig.Clusters[clusterName]

	// -------------------------
	// 3. Build Dev Kubeconfig
	// -------------------------

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
		return err
	}

	fmt.Println("Kubeconfig written to:", fileName)

	return nil
}
