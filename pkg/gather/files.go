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
)

type RemoteDirectory struct {
	Kubeconfig string
	Context    string
	Namespace  string
	Pod        string
	Container  string
	Log        *zap.SugaredLogger
}

var tarFileChangedError *regexp.Regexp

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

	d.log("Starting remote tar: %s", remoteTar)
	err = remoteTar.Start()
	if err != nil {
		return err
	}

	d.log("Starting local tar: %s", localTar)
	err = localTar.Start()
	if err != nil {
		remoteTar.Process.Kill()
		remoteTar.Wait()
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
		d.Pod,
		"--namespace=" + d.Namespace,
		"--container=" + d.Container,
	}

	if d.Kubeconfig != "" {
		args = append(args, "--kubeconfig="+d.Kubeconfig)
	}
	if d.Context != "" {
		args = append(args, "--context="+d.Context)
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

func (d *RemoteDirectory) pathComponents(s string) int {
	sep := string(os.PathSeparator)
	trimmed := strings.Trim(s, sep)
	return strings.Count(trimmed, sep) + 1
}

func (d *RemoteDirectory) log(fmt string, args ...interface{}) {
	if d.Log != nil {
		d.Log.Debugf(fmt, args...)
	}
}

func init() {
	tarFileChangedError = regexp.MustCompile(`(?im)^tar: .+ file changed as we read it$`)
}
