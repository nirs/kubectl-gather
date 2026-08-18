package commands

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"

	"go.uber.org/zap"
)

// FailedToStart is Result.ExitCode when the process failed to start.
const FailedToStart = -2

// Result holds the outcome of a command run by Run.
// Stdout and Stderr are what the process wrote.
// ExitCode == 0 and Err == nil means the process exited successfully.
// ExitCode == -1 means the process was terminated by a signal.
// ExitCode == FailedToStart means the process failed to start.
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Err      error
}

// Run a command, logging lines from stderr and collecting output.
func Run(cmd *exec.Cmd, log *zap.SugaredLogger) Result {
	var stdout, stderr bytes.Buffer

	log.Debugf("Running %v", cmd)

	cmd.Stdout = &stdout

	pipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{Err: err, ExitCode: FailedToStart}
	}

	if err := cmd.Start(); err != nil {
		return Result{Err: err, ExitCode: FailedToStart}
	}

	reader := bufio.NewReader(pipe)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Debugf("Failed to read from command stderr: %s", err)
			}
			break
		}
		log.Debug(string(line))
		stderr.Write(line)
		stderr.WriteByte('\n')
	}

	err = cmd.Wait()
	result := Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
		Err:    err,
	}
	if ee, ok := err.(*exec.ExitError); ok {
		result.ExitCode = ee.ExitCode()
	}

	return result
}

func Stderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
