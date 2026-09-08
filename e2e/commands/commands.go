package commands

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"

	"go.uber.org/zap"
)

// Result holds the outcome of a command run by Run.
type Result struct {
	ExitCode int
	Stdout   bytes.Buffer
	Stderr   bytes.Buffer
	Err      error
}

// Run a command, logging lines from stderr and collecting output in Result.
func Run(cmd *exec.Cmd, log *zap.SugaredLogger) Result {
	var result Result

	log.Debugf("Running %v", cmd)

	cmd.Stdout = &result.Stdout

	pipe, err := cmd.StderrPipe()
	if err != nil {
		result.Err = err
		return result
	}

	if err := cmd.Start(); err != nil {
		result.Err = err
		return result
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
		result.Stderr.Write(line)
		result.Stderr.WriteByte('\n')
	}

	err = cmd.Wait()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			result.ExitCode = ee.ExitCode()
		}
		result.Err = err
	}

	return result
}

func Stderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
