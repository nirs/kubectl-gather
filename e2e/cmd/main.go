package main

import (
	"log"
	"os"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "e2e",
	Short: "Manage the e2e testing environment",
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the e2e environment",
	Run: func(cmd *cobra.Command, args []string) {
		if err := clusters.Create(); err != nil {
			log.Fatal(err)
		}
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the e2e environment",
	Run: func(cmd *cobra.Command, args []string) {
		if err := clusters.Delete(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(createCmd)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
