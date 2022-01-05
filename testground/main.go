
package main

import (
	"github.com/testground/sdk-go/run"

	"github.com/wetware/lab/testground/tests"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"pex": tests.RunPex,
	})
}
