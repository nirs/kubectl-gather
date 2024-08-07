// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

type RemoteDirectory struct {
	pod  *corev1.Pod
	opts *Options
	log  *zap.SugaredLogger
}

var tarFileChangedError *regexp.Regexp

func NewRemoteDirectory(pod *corev1.Pod, opts *Options, log *zap.SugaredLogger) *RemoteDirectory {
	return &RemoteDirectory{pod: pod, opts: opts, log: log}
}

func (d *RemoteDirectory) Gather(src string, dst string) error {
	// We run remote tar and pipe the output to local tar:
	// kubectl exec ... -- tar cf - src | tar xf - -C dst

	var remoteError bytes.Buffer
	remoteTar := d.remoteTarCommand(src)
	remoteTar.Stderr = &remoteError

	pipe, err := remoteTar.StdoutPipe()
	if err != nil {
		return err
	}

	var localError bytes.Buffer
	localTar := d.localTarCommand(dst, d.pathComponents(src))
	localTar.Stderr = &localError
	localTar.Stdin = pipe

	d.log.Debugf("Starting remote tar: %s", remoteTar)
	err = remoteTar.Start()
	if err != nil {
		return err
	}

	d.log.Debugf("Starting local tar: %s", localTar)
	err = localTar.Start()
	if err != nil {
		d.silentTerminate(remoteTar)
		return err
	}

	// Order is important: Must wait for local tar first - if the remote tar
	// fails, the local tar exit. However if the local tar fails, the remote tar
	// blocks forever.
	localErr := localTar.Wait()
	remoteErr := remoteTar.Wait()

	if remoteErr != nil {
		stderr := remoteError.String()
		if !d.isFileChangedError(remoteErr, stderr) {
			return fmt.Errorf("remote tar error: %s: %q", remoteErr, stderr)
		}
	}

	if localErr != nil {
		return fmt.Errorf("local tar error: %s: %q", localErr, localError.String())
	}

	return nil
}

func (d *RemoteDirectory) isFileChangedError(err error, stderr string) bool {
	// tar may fail with exitcode 1 only when a file changed while copying it.
	// This is expected condition for log files so we must ignore it.  However,
	// kubectl also fails with exit code 1, for example if the pod is not found
	// so we cannot rely on the exit code.
	exitErr, ok := err.(*exec.ExitError)
	return ok && exitErr.ExitCode() == 1 && tarFileChangedError.MatchString(stderr)
}

func (d *RemoteDirectory) remoteTarCommand(src string) *exec.Cmd {
	args := []string{
		"exec",
		d.pod.Name,
		"--namespace=" + d.pod.Namespace,
		"--container=" + d.pod.Spec.Containers[0].Name,
	}

	if d.opts.Kubeconfig != "" {
		args = append(args, "--kubeconfig="+d.opts.Kubeconfig)
	}
	if d.opts.Context != "" {
		args = append(args, "--context="+d.opts.Context)
	}
	args = append(args, "--", "tar", "cf", "-", src)

	return exec.Command("kubectl", args...)
}

func (d *RemoteDirectory) localTarCommand(dst string, strip int) *exec.Cmd {
	args := []string{
		"xf",
		"-",
		"--directory=" + dst,
		"--strip-components=" + strconv.Itoa(strip),
	}

	return exec.Command("tar", args...)
}

func (d *RemoteDirectory) silentTerminate(cmd *exec.Cmd) {
	if err := cmd.Process.Kill(); err != nil {
		d.log.Warnf("Cannot kill command %v: %s", cmd, err)
		return
	}

	// Command was terminated by signal 9.
	_ = cmd.Wait()
}

func (d *RemoteDirectory) pathComponents(s string) int {
	sep := string(os.PathSeparator)
	trimmed := strings.Trim(s, sep)
	return strings.Count(trimmed, sep) + 1
}

func init() {
	tarFileChangedError = regexp.MustCompile(`(?im)^tar: .+ file changed as we read it$`)
}
