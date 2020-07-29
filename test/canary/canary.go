package canary

import "github.com/testground/sdk-go/runtime"

// RunTest .
func RunTest(runenv *runtime.RunEnv) (error) {
	runenv.RecordSuccess()
	return nil
}