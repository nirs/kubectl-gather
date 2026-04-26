package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/nirs/kubectl-gather/e2e/clusters"
	"github.com/nirs/kubectl-gather/e2e/logging"
)

var log *zap.SugaredLogger

var rootCmd = &cobra.Command{
	Use:   "e2e",
	Short: "Manage the e2e testing environment",
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create the e2e environment",
	Run: func(cmd *cobra.Command, args []string) {
		if err := clusters.Create(log); err != nil {
			log.Fatal(err)
		}
	},
}

var loadCmd = &cobra.Command{
	Use:   "load archive",
	Short: "Load image archive into e2e clusters",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := clusters.Load(log, args[0]); err != nil {
			log.Fatal(err)
		}
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the e2e environment",
	Run: func(cmd *cobra.Command, args []string) {
		if err := clusters.Delete(log); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(loadCmd)
}

func main() {
	var err error
	log, err = logging.NewLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
