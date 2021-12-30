
package main

import (
	"github.com/testground/sdk-go/run"

	"github.com/wetware/lab/tests/pex"
)

func main() {
	run.InvokeMap(map[string]interface{}{
		"pex-convergence": pex.RunConvergence,
		"pex-convergence-matrix": pex.RunConvergenceMatrix,
		"dns-test": pex.RunDnsTest,
	})
}
