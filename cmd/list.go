package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List developer namespaces",
	Run: func(cmd *cobra.Command, args []string) {

		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			panic(err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			panic(err)
		}

		namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			panic(err)
		}

		for _, ns := range namespaces.Items {
			if len(ns.Name) > 4 && ns.Name[:4] == "dev-" {
				fmt.Println(ns.Name)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
