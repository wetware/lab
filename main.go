// Welcome, testground plan writer!
// If you are seeing this for the first time, check out our documentation!
// https://app.gitbook.com/@protocol-labs/s/testground/

package main

import (
	"errors"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"stub": stub,
		// "pex": pex.Run,
		// "routing": routing.Run,
	})
}

func stub(env *runtime.RunEnv, initCtx *run.InitContext) error {
	return errors.New("it works!")
}
