// SPDX-FileCopyrightText: The kubectl-gather authors
// SPDX-License-Identifier: Apache-2.0

package gather

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func NewLogger(verbose bool, names ...string) *log.Logger {
	if !verbose {
		return log.New(io.Discard, "", 0)
	}

	// Command:	gather:
	// Cluster:	gather/dr1:
	// Addon:	gather/dr1/rook-ceph:
	components := append([]string{"gather"}, names...)
	prefix := fmt.Sprintf("%s: ", strings.Join(components, "/"))

	return log.New(os.Stderr, prefix, log.LstdFlags|log.Lmicroseconds)
}
