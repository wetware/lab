// Welcome, testground plan writer!
// If you are seeing this for the first time, check out our documentation!
// https://app.gitbook.com/@protocol-labs/s/testground/

package main

import (
	"github.com/testground/sdk-go/run"

	"github.com/wetware/lab/tests/pex"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"pex/convergence": pex.RunConvergence,
		"pex/resiliency":  pex.RunResiliency,
		"pex/fanout":      pex.RunFanout,
		"pex/strategy":    pex.RunStrategy,
	})
}
