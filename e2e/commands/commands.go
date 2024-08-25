package commands

import (
	"bufio"
	"io"
	"log"
	"os/exec"
)

func LogStderr(cmd *exec.Cmd) error {
	log.Printf("Running %v", cmd)
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
				log.Printf("Failed to read from command stderr: %s", err)
			}
			break
		}
		log.Print(string(line))
	}
	return cmd.Wait()
}

func Stderr(err error) []byte {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.Stderr
	}
	return nil
}
