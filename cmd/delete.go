package cmd

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [username]",
	Short: "Delete developer environment",
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

		err = clientset.CoreV1().
			Namespaces().
			Delete(context.Background(), namespace, metav1.DeleteOptions{})

		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Println("Namespace does not exist:", namespace)
				return
			}
			panic(err)
		}

		fmt.Println("Deleted namespace:", namespace)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
