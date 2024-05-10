// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

type RemoteCommand struct {
	pod       *corev1.Pod
	opts      *Options
	log       *zap.SugaredLogger
	directory string
}

var specialCharacters *regexp.Regexp

func NewRemoteCommand(pod *corev1.Pod, opts *Options, log *zap.SugaredLogger, directroy string) *RemoteCommand {
	return &RemoteCommand{pod: pod, opts: opts, log: log, directory: directroy}
}

func (c *RemoteCommand) Gather(command ...string) error {
	start := time.Now()

	args := []string{
		"exec",
		c.pod.Name,
		"--container=" + c.pod.Spec.Containers[0].Name,
		"--namespace=" + c.pod.Namespace,
	}
	if c.opts.Kubeconfig != "" {
		args = append(args, "--kubeconfig="+c.opts.Kubeconfig)
	}
	if c.opts.Context != "" {
		args = append(args, "--context="+c.opts.Context)
	}
	args = append(args, "--")
	args = append(args, command...)

	filename := c.Filename(command...)
	writer, err := os.Create(filepath.Join(c.directory, filename))
	if err != nil {
		return err
	}

	defer writer.Close()
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = writer

	c.log.Debugf("Running command %s", cmd)
	err = cmd.Run()
	c.log.Debugf("Gathered %s in %.3f seconds", filename, time.Since(start).Seconds())

	return err
}

func (c *RemoteCommand) Filename(command ...string) string {
	name := strings.Join(command, " ")
	return specialCharacters.ReplaceAllString(name, "-")
}

func init() {
	specialCharacters = regexp.MustCompile(`[^\w\.\/]+`)
}
