//go:build nodeidentitycrashtest

package nodeidentity

import "os"

const crashTestEnvironment = "TP_NODE_IDENTITY_TEST_CRASH_AT"

func crashAt(point string) {
	if os.Getenv(crashTestEnvironment) == point {
		os.Exit(86)
	}
}
