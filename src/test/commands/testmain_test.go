package commands_test

import (
	"os"
	"testing"

	"nvm/test/clitest"
)

func TestMain(m *testing.M) {
	if exitCode, handled := clitest.RunSyncTestHelperIfRequested(); handled {
		os.Exit(exitCode)
	}
	if exitCode, handled := clitest.RunReshimTestHelperIfRequested(); handled {
		os.Exit(exitCode)
	}
	os.Exit(m.Run())
}
