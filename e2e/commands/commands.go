package commands

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os/exec"
)

// Run a command, logging and capturing lines from stderr.
func Run(cmd *exec.Cmd) (string, error) {
	log.Printf("Running %v", cmd)
	var stderr bytes.Buffer
	pipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	reader := bufio.NewReader(pipe)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read from command stderr: %s", err)
			}
			break
		}
		log.Print(string(line))
		stderr.Write(line)
		stderr.WriteByte('\n')
	}
	return stderr.String(), cmd.Wait()
}

func Stderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
