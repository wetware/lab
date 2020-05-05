package main

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/event"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/lthibault/wetware/pkg/server"
	"github.com/lthibault/ww-test-plans/testutil"
)

// Announce verifies that hosts are mutually aware of each others' presence.
func Announce(runenv *runtime.RunEnv) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	host := server.New(
		server.WithLogger(testutil.ZapLogger(runenv)),
		server.WithDiscover(&testutil.Discover{
			RunEnv: runenv,
			Client: client,
		}),
	)

	bus := host.EventBus()
	sub, err := bus.Subscribe(new(event.EvtLocalAddressesUpdated))
	if err != nil {
		return err
	}
	defer sub.Close()

	if err = host.Start(); err != nil {
		return errors.Wrap(err, "start host")
	}
	defer host.Close()

	select {
	case <-sub.Out():
		// Listen addr has been bound.  Wait a few moments for the announce loop to
		// start and for heartbeats to propagate.
		time.Sleep(time.Millisecond * 100)
	case <-ctx.Done():
		return ctx.Err()
	}

	// tests proper
	peers := host.Peers()
	runenv.SLogger().With("peers", peers).Debug("found peers")

	switch {
	case len(peers) != runenv.TestInstanceCount:
		err = errors.Errorf("expected %d peers, found %d",
			runenv.TestInstanceCount,
			len(peers))
	case !contains(peers, host.ID()):
		err = errors.Errorf("%s not in peer set", host.ID())
	case !duplicates(peers):
		err = errors.New("duplicates detected")
	}

	if err != nil {
		runenv.RecordFailure(err)
	} else {
		runenv.RecordSuccess()
	}

	return err
}

func contains(ps []peer.ID, p peer.ID) bool {
	set := make(map[peer.ID]struct{})
	for _, px := range ps {
		set[px] = struct{}{}
	}

	_, ok := set[p]
	return ok
}

func duplicates(ps []peer.ID) bool {
	set := make(map[peer.ID]struct{})
	for _, px := range ps {
		set[px] = struct{}{}
	}

	return len(ps) != len(set)
}
