// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nirs/kubectl-gather/pkg/gather"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var directory string
var kubeconfig string
var contexts []string
var namespaces []string
var verbose bool

var example = `  # Gather data from all namespaces in current context in my-kubeconfig and
  # store it in gather-{timestamp}.
  kubectl gather --kubeconfig my-kubeconfig

  # Gather data from all namespaces in clusters "dr1", "dr2" and "hub" and store
  # it in "gather/", using default kubeconfig (~/.kube/config).
  kubectl gather --directory gather --contexts dr1,dr2,hub

  # Gather data from namespaces "my-ns" and "other-ns" in cluster "dr1" and
  # store it in gather/, using default kubeconfig (~/.kube/config).
  kubectl gather --directory gather --contexts dr1 --namespaces my-ns,other-ns`

var rootCmd = &cobra.Command{
	Use:     "kubectl-gather",
	Short:   "Gather data from clusters",
	Example: example,
	Annotations: map[string]string{
		cobra.CommandDisplayNameAnnotation: "kubectl gather",
	},
	Run: gatherAll,
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
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig(),
		"the kubeconfig file to use")
	rootCmd.Flags().StringSliceVar(&contexts, "contexts", nil,
		"comma separated list of contexts to gather data from")
	rootCmd.Flags().StringSliceVarP(&namespaces, "namespaces", "n", nil,
		"comma separated list of namespaces to gather data from")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false,
		"be more verbose")
}

type result struct {
	Count int32
	Err   error
}

func gatherAll(cmd *cobra.Command, args []string) {
	start := time.Now()

	if directory == "" {
		directory = defaultGatherDirectory()
	}

	log := createLogger()
	defer log.Sync()

	log.Infof("Using kubeconfig %q", kubeconfig)
	config, err := loadConfig(kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(contexts) == 0 {
		if config.CurrentContext == "" {
			log.Fatal("No context specified and current context not set")
		}

		log.Infof("Using current context %q", config.CurrentContext)
		contexts = append(contexts, config.CurrentContext)
	}

	if err := validateContexts(config, contexts); err != nil {
		log.Fatalf("Invalid contexts: %s", err)
	}

	if len(namespaces) != 0 {
		log.Infof("Gathering from namespaces %v", namespaces)
	} else {
		log.Infof("Gathering from all namespaces")
	}

	if !cmd.Flags().Changed("directory") {
		log.Infof("Storing data in %q", directory)
	}

	wg := sync.WaitGroup{}
	results := make(chan result, len(contexts))

	for _, context := range contexts {
		log.Infof("Gathering cluster %q", context)

		directory := filepath.Join(directory, context)

		options := gather.Options{
			Kubeconfig: kubeconfig,
			Context:    context,
			Namespaces: namespaces,
			Log:        log.Named(context),
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()

			restConfig, err := clientcmd.NewNonInteractiveClientConfig(
				*config, options.Context, nil, nil).ClientConfig()
			if err != nil {
				results <- result{Err: err}
				return
			}

			g, err := gather.New(restConfig, directory, options)
			if err != nil {
				results <- result{Err: err}
				return
			}

			err = g.Gather()
			results <- result{Count: g.Count(), Err: err}
			if err != nil {
				return
			}

			log.Infof("Gathered %d resources from cluster %q in %.3f seconds",
				g.Count(), options.Context, time.Since(start).Seconds())

		}()
	}

	wg.Wait()
	close(results)

	count := int32(0)

	for r := range results {
		if r.Err != nil {
			log.Fatal(r.Err)
		}
		count += r.Count
	}

	if len(namespaces) != 0 && count == 0 {
		// Likely a user error like a wrong namespace.
		log.Warnf("No resource gathered from namespaces %v", namespaces)
	}

	log.Infof("Gathered %d resources from %d clusters in %.3f seconds",
		count, len(contexts), time.Since(start).Seconds())
}

func createLogger() *zap.SugaredLogger {
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

func validateContexts(config *api.Config, contexts []string) error {
	for _, context := range contexts {
		ctx, ok := config.Contexts[context]
		if !ok {
			return fmt.Errorf("context %q does not exist", context)
		}

		if _, ok := config.Clusters[ctx.Cluster]; !ok {
			return fmt.Errorf("context %q does not have a cluster", context)
		}

		if _, ok := config.AuthInfos[ctx.AuthInfo]; !ok {
			return fmt.Errorf("context %q does not have a auth info", context)
		}
	}

	return nil
}

func defaultKubeconfig() string {
	env := os.Getenv("KUBECONFIG")
	if env != "" {
		return env
	}
	return clientcmd.RecommendedHomeFile
}

func loadConfig(kubeconfig string) (*api.Config, error) {
	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func defaultGatherDirectory() string {
	return time.Now().Format("gather.20060102150405")
}
