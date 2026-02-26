package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "v0.1.2"

var kubeconfig string

var rootCmd = &cobra.Command{
	Use:   "podcraft",
	Short: "PodCraft - Lightweight Kubernetes Internal Developer Platform",
	Long: `
PodCraft is a lightweight, infra-agnostic Kubernetes Internal Developer Platform (IDP).

It provisions isolated, resource-governed, zero-trust namespaces for developers
using native Kubernetes APIs.

Core Concepts:
  • Namespace-per-developer isolation
  • Least-privilege RBAC
  • Zero-trust NetworkPolicies
  • ResourceQuota and LimitRange enforcement
  • Optional persistent storage (PVC with quota)
  • Developer-scoped kubeconfig generation

PodCraft is safe to re-run (idempotent) and reconciles drift automatically.

Examples:

  Create developer environment:
    podcraft create alice

  Create with custom limits:
    podcraft create alice --cpu=4 --memory=4Gi --max-pods=20

  Delete developer environment:
    podcraft delete alice

  Describe developer namespace:
    podcraft describe alice

Use "podcraft [command] --help" for more information about a command.
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&kubeconfig,
		"kubeconfig",
		os.Getenv("HOME")+"/.kube/config",
		"Path to admin kubeconfig file",
	)

	rootCmd.SetVersionTemplate("PodCraft {{.Version}}\n")
	rootCmd.Version = Version
}
