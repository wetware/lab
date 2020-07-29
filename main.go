// Welcome, testground plan writer!
// If you are seeing this for the first time, check out our documentation!
// https://app.gitbook.com/@protocol-labs/s/testground/

package main

import (
	"github.com/pkg/errors"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"

	"github.com/wetware/lab/test/announce"
	"github.com/wetware/lab/test/canary"
)

func main() {
	run.Invoke(func(runenv *runtime.RunEnv) error {
		switch c := runenv.TestCase; c {
		case "canary":
			return canary.RunTest(runenv)
		case "announce":
			return announce.RunTest(runenv)
		default:
			return errors.Errorf("Unknown Testcase %s", c)
		}
	})
}
