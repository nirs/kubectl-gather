// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type RemoteCommand struct {
	Kubeconfig string
	Context    string
	Namespace  string
	Pod        string
	Container  string
	Directory  string
}

var specialCharacters *regexp.Regexp

func (c *RemoteCommand) Gather(command ...string) error {
	args := []string{"exec", c.Pod}
	if c.Kubeconfig != "" {
		args = append(args, "--kubeconfig="+c.Kubeconfig)
	}
	if c.Context != "" {
		args = append(args, "--context="+c.Context)
	}
	if c.Namespace != "" {
		args = append(args, "--namespace="+c.Namespace)
	}
	if c.Container != "" {
		args = append(args, "--container="+c.Container)
	}
	args = append(args, "--")
	args = append(args, command...)

	writer, err := os.Create(c.Filename(command...))
	if err != nil {
		return err
	}

	defer writer.Close()
	cmd := exec.Command("kubectl", args...)
	cmd.Stdout = writer

	return cmd.Run()
}

func (c *RemoteCommand) Filename(command ...string) string {
	name := strings.Join(command, " ")
	name = specialCharacters.ReplaceAllString(name, "-")
	return filepath.Join(c.Directory, name)
}

func init() {
	specialCharacters = regexp.MustCompile(`[^\w\.\/]+`)
}
