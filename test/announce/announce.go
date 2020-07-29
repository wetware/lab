package announce

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/wetware/ww/pkg/client"
	"github.com/wetware/ww/pkg/host"

	lab "github.com/wetware/lab/pkg"
)

var (
	stateInit  = sync.State("init")
	stateReady = sync.State("ready")
	stateDone  = sync.State("done")
)

// RunTest tests cluster-wise peer announcement.  It verifies that hosts are mutually
// aware of each others' presence.
func RunTest(runenv *runtime.RunEnv, initc *run.InitContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	switch initc.SyncClient.MustSignalAndWait(ctx, stateInit, runenv.TestInstanceCount) {
	case 1:
		return subscribeClient(ctx, runenv, initc)
	default:
		return announceHost(ctx, runenv, initc)
	}
}

func subscribeClient(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	runenv.RecordMessage("I am the client")
	defer initc.SyncClient.MustSignalEntry(context.Background(), sync.State("done"))

	c, err := dial(ctx, runenv, initc)
	if err != nil {
		return err
	}
	defer c.Close()

	topic, err := c.Join("")
	if err != nil {
		return err
	}
	defer topic.Close()

	sub, err := topic.Subscribe(ctx)
	if err != nil {
		return err
	}

	ps := make(map[peer.ID]struct{})
	for msg := range sub.C {
		if _, ok := ps[msg.GetFrom()]; ok {
			continue
		}

		runenv.RecordMessage("got entry for %s", msg.GetFrom())

		// loop until at least one message from all peers was found.
		if ps[msg.GetFrom()] = struct{}{}; len(ps) == runenv.TestInstanceCount-1 {
			break
		}
	}

	return nil
}

func announceHost(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b := &lab.Bootstrapper{
		RunEnv:     runenv,
		SyncClient: initc.SyncClient,
	}

	host, err := host.New(host.WithBootStrategy(b))
	if err != nil {
		return err
	}
	defer host.Close()

	runenv.RecordMessage("%s ready", host.ID())

	initc.SyncClient.MustSignalEntry(ctx, stateReady)   // tell client we're good to go
	<-initc.SyncClient.MustBarrier(ctx, stateDone, 1).C // wait for client to terminate

	return nil
}

func dial(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) (c client.Client, err error) {
	// Wait for at least one host to be available.  We're purposefully playing fast and
	// loose to test dynamic joining of new hosts to an existing cluster.
	ready := initc.SyncClient.MustBarrier(ctx, stateReady, 1)

	select {
	case err = <-ready.C:
		b := &lab.Bootstrapper{
			RunEnv:     runenv,
			SyncClient: initc.SyncClient,
		}

		c, err = client.Dial(ctx, client.WithBootStrategy(b))
	case <-ctx.Done():
		err = ctx.Err()
	}

	return
}
