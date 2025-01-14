// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/nirs/kubectl-gather/pkg/gather"
)

var directory string
var kubeconfig string
var contexts []string
var namespaces []string
var addons []string
var remote bool
var verbose bool
var log *zap.SugaredLogger

var example = `  # Gather data from all namespaces in current context in my-kubeconfig and
  # store it in gather.{timestamp}.
  kubectl gather --kubeconfig my-kubeconfig

  # Gather data from all namespaces in clusters "dr1", "dr2" and "hub" and store
  # it in "gather.local/", using default kubeconfig (~/.kube/config).
  kubectl gather --contexts dr1,dr2,hub --directory gather.local

  # Gather data from namespaces "my-ns" and "other-ns" in clusters "dr1", "dr2",
  # and "hub", and store it in "gather.ns/".
  kubectl gather --contexts dr1,dr2,hub --namespaces my-ns,other-ns --directory gather.ns

  # Gather data on the remote clusters "dr1", "dr2" and "hub" and download it to
  # "gather.remote/". Requires the "oc" command.
  kubectl gather --contexts dr1,dr2,hub --remote --directory gather.remote

  # Enable only the "logs" addon, gathering all resources and pod logs. Use
  # --addons= to disable all addons.
  kubectl gather --contexts dr1,dr2,hub --addons logs --directory gather.resources+logs`

var rootCmd = &cobra.Command{
	Use:     "kubectl-gather",
	Short:   "Gather data from clusters",
	Version: gather.Version,
	Example: example,
	Annotations: map[string]string{
		cobra.CommandDisplayNameAnnotation: "kubectl gather",
	},
	Run: runGather,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&directory, "directory", "d", "",
		"directory for storing gathered data (default \"gather.{timestamp}\")")

	// Don't set default kubeconfig, so kubeconfig is empty unless the user
	// specified the option. This is required to allow running remote commands
	// using in-cluster config.
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "",
		"the kubeconfig file to use")

	rootCmd.Flags().StringSliceVar(&contexts, "contexts", nil,
		"comma separated list of contexts to gather data from")
	rootCmd.Flags().StringSliceVarP(&namespaces, "namespaces", "n", nil,
		"if specified, comma separated list of namespaces to gather data from")
	rootCmd.Flags().StringSliceVar(&addons, "addons", nil,
		fmt.Sprintf("if specified, comma separated list of addons to enable (available addons: %s)",
			availableAddons()))
	rootCmd.Flags().BoolVarP(&remote, "remote", "r", false,
		"run on the remote clusters (requires the \"oc\" command)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"be more verbose")

	// Use plain, machine friendly version string.
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func runGather(cmd *cobra.Command, args []string) {
	if directory == "" {
		directory = defaultGatherDirectory()
	}

	log = createLogger(directory, verbose)
	defer func() {
		_ = log.Sync()
	}()

	clusters, err := loadClusterConfigs(contexts, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(namespaces) != 0 {
		log.Infof("Gathering from namespaces %q", namespaces)
	} else {
		log.Infof("Gathering from all namespaces")
	}

	if addons != nil {
		log.Infof("Using addons %q", addons)
	} else {
		log.Infof("Using all addons")
	}

	if !cmd.Flags().Changed("directory") {
		log.Infof("Storing data in %q", directory)
	}

	if remote {
		remoteGather(clusters)
	} else {
		localGather(clusters)
	}
}

func createLogger(directory string, verbose bool) *zap.SugaredLogger {
	if err := os.MkdirAll(directory, 0750); err != nil {
		stdlog.Fatalf("Cannot create directory: %s", err)
	}

	logfile, err := os.Create(filepath.Join(directory, "gather.log"))
	if err != nil {
		stdlog.Fatalf("Cannot create log file: %s", err)
	}

	config := zap.NewDevelopmentEncoderConfig()
	config.CallerKey = ""
	encoder := zapcore.NewConsoleEncoder(config)

	consoleLevel := zapcore.InfoLevel
	if verbose {
		consoleLevel = zapcore.DebugLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.Lock(logfile), zapcore.DebugLevel),
		zapcore.NewCore(encoder, zapcore.Lock(os.Stderr), consoleLevel),
	)

	return zap.New(core).Named("gather").Sugar()
}

func defaultGatherDirectory() string {
	return time.Now().Format("gather.20060102150405")
}

func availableAddons() string {
	names := gather.AvailableAddons()
	slices.Sort(names)
	return strings.Join(names, ", ")
}
