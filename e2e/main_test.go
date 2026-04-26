//go:build e2e

package e2e

import (
	"testing"

	"github.com/nirs/kubectl-gather/e2e/test"
)

func TestMain(m *testing.M) {
	test.Main(m)
}
