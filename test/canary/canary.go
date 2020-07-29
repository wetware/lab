package canary

import (
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

// RunTest .
func RunTest(runenv *runtime.RunEnv, initc *run.InitContext) error {
	runenv.RecordSuccess()
	return nil
}
