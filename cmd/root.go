// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/base64"
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
var cluster bool
var remote bool
var salt string
var parsedSalt gather.Salt
var verbose bool
var logFormat string
var mustGatherVersion bool
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
  # "gather.remote/". Requires the "oc" command. Use --salt to ensure all remote
  # clusters use the same salt for consistent secret hashing.
  kubectl gather --contexts dr1,dr2,hub --remote --salt "$(openssl rand -base64 16)" --directory gather.remote

  # Enable only the "logs" addon, gathering all resources and pod logs. Use
  # --addons= to disable all addons.
  kubectl gather --contexts dr1,dr2,hub --addons logs --directory gather.resources+logs

  # Gather both cluster and namespace resources from "my-ns" and "other-ns" in clusters
  # "dr1", "dr2", and "hub".
  kubectl gather --contexts dr1,dr2,hub --namespaces my-ns,other-ns --cluster --directory gather.mixed

  # Gather only cluster resources from clusters "dr1", "dr2" and "hub".
  kubectl gather --contexts dr1,dr2,hub --namespace="" --cluster --directory gather.cluster`

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
	rootCmd.Flags().BoolVar(&cluster, "cluster", false,
		"if true, gather cluster scoped resources, if namespaces and cluster flags are not "+
			"specified, gather all resources")
	rootCmd.Flags().BoolVarP(&remote, "remote", "r", false,
		"run on the remote clusters (requires the \"oc\" command)")
	rootCmd.Flags().StringVar(&salt, "salt", "",
		"base64-encoded 16-byte salt for secret hashing (default: randomly generated)")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"be more verbose")
	rootCmd.Flags().StringVar(&logFormat, "log-format", "text", "Set the logging format [text, json]")
	rootCmd.Flags().BoolVar(&mustGatherVersion, "must-gather-version", false,
		"print must-gather version info and exit")

	// Use plain, machine friendly version string.
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func runGather(cmd *cobra.Command, args []string) {
	if mustGatherVersion {
		fmt.Printf("kubectl-gather\nv%s\n", gather.Version)
		return
	}

	if directory == "" {
		directory = defaultGatherDirectory()
	}

	log = createLogger(directory, verbose, logFormat)
	defer func() {
		_ = log.Sync()
	}()

	if err := validateOptions(cmd); err != nil {
		log.Fatal(err)
	}

	clusterConfigs, err := loadClusterConfigs(contexts, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	if cmd.Flags().Changed("salt") {
		log.Infof("Using user-provided salt %q", salt)
	} else {
		log.Infof("Using generated salt %q", salt)
	}

	if namespaces == nil {
		log.Infof("Gathering from all namespaces")
	} else if len(namespaces) > 0 {
		log.Infof("Gathering from namespaces %q", namespaces)
	}

	if cluster {
		log.Info("Gathering cluster scoped resources")
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
		remoteGather(clusterConfigs)
	} else {
		localGather(clusterConfigs)
	}
}

func validateOptions(cmd *cobra.Command) error {
	if salt != "" {
		var err error
		parsedSalt, err = validateSalt(salt)
		if err != nil {
			return err
		}
	} else {
		parsedSalt = gather.RandomSalt()
		// Keep the base64 salt string for logging and passing to remote clusters.
		salt = base64.StdEncoding.EncodeToString(parsedSalt[:])
	}

	// --namespaces=""
	// --namespaces="" --cluster=false
	if namespaces != nil && len(namespaces) == 0 && !cluster {
		return fmt.Errorf("nothing to gather: specify --namespaces or --cluster")
	}

	// --namespaces and --cluster flags are not set
	if !cmd.Flags().Changed("namespaces") && !cmd.Flags().Changed("cluster") {
		cluster = true
	}

	return nil
}

func createLogger(directory string, verbose bool, format string) *zap.SugaredLogger {
	consoleConfig := zap.NewProductionEncoderConfig()
	logfileConfig := zap.NewProductionEncoderConfig()

	// Use formatted timestamps instead of seconds since epoch to make it easier
	// to related to other logs.
	consoleConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logfileConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Use UPPERCASE log levels to make it easier to read.
	consoleConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	logfileConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	var consoleEncoder zapcore.Encoder
	var logfileEncoder zapcore.Encoder

	switch format {
	case "text":
		// Caller and stacktraces are useless noise in the console text logs,
		// but may be helpful in json format when the logs are consumed by
		// another program.
		consoleConfig.CallerKey = zapcore.OmitKey
		consoleConfig.StacktraceKey = zapcore.OmitKey
		consoleEncoder = zapcore.NewConsoleEncoder(consoleConfig)
		// In the log file caller and stacktraces are nice to have.
		logfileEncoder = zapcore.NewConsoleEncoder(logfileConfig)
	case "json":
		// When using json logs we want all possible info, so a program can
		// consume what it needs.
		consoleEncoder = zapcore.NewJSONEncoder(consoleConfig)
		logfileEncoder = zapcore.NewJSONEncoder(logfileConfig)
	default:
		stdlog.Fatalf("Invalid log-format: %q", format)
	}

	if err := os.MkdirAll(directory, 0750); err != nil {
		stdlog.Fatalf("Cannot create directory: %s", err)
	}

	logfile, err := os.Create(filepath.Join(directory, "gather.log"))
	if err != nil {
		stdlog.Fatalf("Cannot create log file: %s", err)
	}

	consoleLevel := zapcore.InfoLevel
	if verbose {
		consoleLevel = zapcore.DebugLevel
	}

	core := zapcore.NewTee(
		zapcore.NewCore(logfileEncoder, zapcore.Lock(logfile), zapcore.DebugLevel),
		zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), consoleLevel),
	)

	return zap.New(core).Named("gather").Sugar()
}

func validateSalt(saltFlag string) (gather.Salt, error) {
	var salt gather.Salt

	decodedSalt, err := base64.StdEncoding.DecodeString(saltFlag)
	if err != nil {
		return salt, fmt.Errorf("invalid --salt value: must be base64-encoded: %w", err)
	}
	if len(decodedSalt) != 16 {
		return salt, fmt.Errorf("invalid --salt value: must be 16 bytes, got %d", len(decodedSalt))
	}

	copy(salt[:], decodedSalt)
	return salt, nil
}

func defaultGatherDirectory() string {
	return time.Now().Format("gather.20060102150405")
}

func availableAddons() string {
	names := gather.AvailableAddons()
	slices.Sort(names)
	return strings.Join(names, ", ")
}
