package pex

import (
	"context"
	"time"

	zaputil "github.com/lthibault/log/util/zap"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/discovery"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/pkg/boot"
)

const ns = "casm.lab.pex"

// Run tests for PeX.
func RunConvergence(env *runtime.RunEnv, initCtx *run.InitContext) error {
	var (
		tick = time.Millisecond*time.Duration(env.IntParam("tick")) // tick in miliseconds
		convTickAmount = env.IntParam("convickAmount")
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := boot.New(env, initCtx)
	if err != nil {
		return err
	}

	h, err := libp2p.New(context.Background())
	if err != nil {
		return err
	}
	defer h.Close()

	px, err := pex.New(h,
		pex.WithNamespace(ns),              // make sure different tests don't interact with each other
		pex.WithSelector(nil),              // change this to test different view seleciton policies
		pex.WithTick(tick), // speed up the simulation
		pex.WithLogger(zaputil.Wrap(env.SLogger())))
	if err != nil {
		return err
	}

	ps, err := d.FindPeers(ctx, ns, discovery.Limit(1))
	if err != nil {
		return err
	}

	for info := range ps {
		if err := px.Join(ctx, info); err != nil {
			return err
		}
	}

	// TODO:  actual test starts here
	// Test 1: How fast does PeX converge on a uniform distribution of records?
	time.Sleep(tick*time.Duration(convTickAmount))


	return nil
}