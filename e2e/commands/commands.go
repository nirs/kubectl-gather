package commands

import (
	"bufio"
	"io"
	"os/exec"

	"go.uber.org/zap"
)

// Run a command logging lines from stderr.
func Run(cmd *exec.Cmd, log *zap.SugaredLogger) error {
	log.Debugf("Running %v", cmd)
	pipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
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
	}
	return cmd.Wait()
}

func Stderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
