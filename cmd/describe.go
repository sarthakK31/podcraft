package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var describeCmd = &cobra.Command{
	Use:   "describe [username]",
	Short: "Describe developer namespace",
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

		// Check namespace exists
		_, err = clientset.CoreV1().
			Namespaces().
			Get(ctx, namespace, metav1.GetOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Println("Namespace does not exist:", namespace)
				return
			}
			panic(err)
		}

		fmt.Println("====================================")
		fmt.Println("Namespace:", namespace)
		fmt.Println("====================================")

		// ResourceQuota
		quota, err := clientset.CoreV1().
			ResourceQuotas(namespace).
			Get(ctx, "dev-quota", metav1.GetOptions{})

		if err == nil {
			fmt.Println("\nResourceQuota:")
			for resource, hard := range quota.Status.Hard {
				used := quota.Status.Used[resource]
				fmt.Printf("  %s: %s / %s\n", resource.String(), used.String(), hard.String())
			}
		}

		// Pods
		pods, err := clientset.CoreV1().
			Pods(namespace).
			List(ctx, metav1.ListOptions{})

		if err == nil {
			fmt.Println("\nPods:")
			if len(pods.Items) == 0 {
				fmt.Println("  No pods running")
			}
			for _, pod := range pods.Items {
				fmt.Printf("  %s (%s)\n", pod.Name, pod.Status.Phase)
			}
		}

		// NetworkPolicies
		netpols, err := clientset.NetworkingV1().
			NetworkPolicies(namespace).
			List(ctx, metav1.ListOptions{})

		if err == nil {
			fmt.Println("\nNetworkPolicies:")
			for _, np := range netpols.Items {
				fmt.Println(" ", np.Name)
			}
		}

		fmt.Println("\n====================================")
		fmt.Println("End of Report")
		fmt.Println("====================================")
	},
}

func init() {
	rootCmd.AddCommand(describeCmd)
}
