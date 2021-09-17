package pex

import (
	"context"
	"time"

	zaputil "github.com/lthibault/log/util/zap"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/discovery"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/pkg/boot"
)

// Run tests for PeX.
func RunResiliency(env *runtime.RunEnv, initCtx *run.InitContext) error {
	var (
		tick           = time.Millisecond * time.Duration(env.IntParam("tick")) // tick in miliseconds
		convTickAmount = env.IntParam("convTickAmount")
		churn          = env.FloatParam("churn") // churn amount in percentage
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// instantiate a sync service client, binding it to the RunEnv.
	client := sync.MustBoundClient(ctx, env)
	defer client.Close()
	// signal entry in the 'enrolled' state, and obtain a sequence number.
	seq := client.MustSignalEntry(ctx, sync.State("enrolled"))

	h, err := libp2p.New(context.Background())
	if err != nil {
		return err
	}
	defer h.Close()

	d, err := boot.New(env, client, h, seq)
	if err != nil {
		return err
	}

	px, err := pex.New(h,
		pex.WithNamespace(ns), // make sure different tests don't interact with each other
		pex.WithSelector(nil), // change this to test different view seleciton policies
		pex.WithTick(tick),    // speed up the simulation
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
	// Test 2: How resilient is the overlay to partial failure without partitions?
	time.Sleep(tick * time.Duration(convTickAmount))
	if seq < int64(float64(env.TestInstanceCount)*churn) {
		h.Close()
	}
	time.Sleep(tick * time.Duration(convTickAmount))

	return nil
}
