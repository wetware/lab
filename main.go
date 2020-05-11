// Welcome, testground plan writer!
// If you are seeing this for the first time, check out our documentation!
// https://app.gitbook.com/@protocol-labs/s/testground/

package main

import (
	"github.com/pkg/errors"

	"github.com/testground/sdk-go/runtime"
)

func main() {
	runtime.Invoke(run)
}

func run(runenv *runtime.RunEnv) error {
	switch c := runenv.TestCase; c {
	case "heartbeat":
		return Heartbeat(runenv)
	case "announce":
		return Announce(runenv)
	default:
		return errors.Errorf("Unknown Testcase %s", c)
	}
}
