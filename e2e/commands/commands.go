package commands

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
)

// Command wraps exec.Cmd with the same Start/Wait/Run interface.
// Unlike exec.Cmd, Wait and Run log every line from stderr. To capture
// stderr, set the Stderr field before calling Start or Run.
type Command struct {
	Stderr io.Writer
	cmd    *exec.Cmd
	pipe   io.ReadCloser
}

// New creates a Command, like exec.Command.
func New(name string, args ...string) *Command {
	return &Command{cmd: exec.Command(name, args...)}
}

// Process returns the underlying os.Process, like exec.Cmd.Process.
// Only available after Start.
func (c *Command) Process() *os.Process {
	return c.cmd.Process
}

// Start starts the command, like exec.Cmd.Start.
func (c *Command) Start() error {
	log.Printf("Running %v", c.cmd)
	pipe, err := c.cmd.StderrPipe()
	if err != nil {
		return err
	}
	c.pipe = pipe
	return c.cmd.Start()
}

// Wait waits for the command to exit, like exec.Cmd.Wait. Unlike
// exec.Cmd.Wait, it logs every line from stderr. If Stderr is set, each
// line is also written there.
func (c *Command) Wait() error {
	reader := bufio.NewReader(c.pipe)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read from command stderr: %s", err)
			}
			break
		}
		log.Print(string(line))
		if c.Stderr != nil {
			c.Stderr.Write(line)
			c.Stderr.Write([]byte{'\n'})
		}
	}
	return c.cmd.Wait()
}

// Run starts the command and waits for it to exit, like exec.Cmd.Run.
// Unlike exec.Cmd.Run, it logs every line from stderr. If Stderr is
// set, each line is also written there.
func (c *Command) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Stderr extracts the stderr output from an exec.ExitError.
func Stderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
