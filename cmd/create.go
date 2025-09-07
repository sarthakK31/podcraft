package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"podcraft/pkg/kube"
	"podcraft/pkg/kubeconfigpkg"
	"podcraft/pkg/namespacepkg"
	"podcraft/pkg/network"
	"podcraft/pkg/quota"
	"podcraft/pkg/rbac"
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

		clientset, err := kube.GetClient(kubeconfig)
		if err != nil {
			panic(err)
		}

		// ctx := context.Background()

		// Creating Namespace (Idempotent)
		err = namespacepkg.EnsureNamespace(clientset, namespace, username)
		if err != nil {
			panic(err)
		}

		// Creating RBAC (Idempotent) - service-account, role, rolebinding
		err = rbac.EnsureRBAC(clientset, namespace, username)
		if err != nil {
			panic(err)
		}

		// Generating kubeconfig for the user and loading Service account token
		err = kubeconfigpkg.Generate(clientset, kubeconfig, namespace, username)
		if err != nil {
			panic(err)
		}

		// Applying Network Policies

		err = network.EnsureNetwork(clientset, namespace)
		if err != nil {
			panic(err)
		}
		fmt.Println("NetworkPolicies applied")

		// Applying ResourceQuota (Hardcoded Defaults)

		err = quota.EnsureQuota(clientset, namespace, cpuLimit, memoryLimit, maxPods)
		if err != nil {
			panic(err)
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
