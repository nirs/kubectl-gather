// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"fmt"
	"io"
	"log"
	"os"
)

func createLogger(name string, opts *Options) *log.Logger {
	if opts.Verbose {
		prefix := fmt.Sprintf("%s/%s: ", opts.Context, name)
		return log.New(os.Stderr, prefix, log.LstdFlags|log.Lmicroseconds)
	}
	return log.New(io.Discard, "", 0)
}
