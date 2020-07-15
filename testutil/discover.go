package testutil

import (
	"context"
	"math/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/lthibault/wetware/pkg/discover"
)

var (
	topic              = sync.NewTopic("discover", new(peer.AddrInfo))
	stateDiscoverReady = sync.State("discover ready")
)

// Discover implements discovery over github.com/testground/sdk-go/sync.
// It does not close the underlying sync.Client.
type Discover struct {
	RunEnv *runtime.RunEnv
	Client *sync.Client

	id peer.ID
}

// DiscoverPeers over Testground sync service.
func (d *Discover) DiscoverPeers(ctx context.Context) ([]peer.AddrInfo, error) {
	b, err := d.Client.Barrier(ctx, stateDiscoverReady, d.RunEnv.TestInstanceCount)
	if err != nil {
		return nil, err
	}

	if err = <-b.C; err != nil {
		return nil, err
	}

	ch := make(chan peer.AddrInfo, 1)
	sub, err := d.Client.Subscribe(ctx, topic, ch)
	if err != nil {
		return nil, err
	}

	addrs := make([]peer.AddrInfo, 0, d.RunEnv.TestInstanceCount-1)
	for {
		select {
		case info := <-ch:
			if info.ID != d.id {
				addrs = append(addrs, info)
			}
		case <-sub.Done():
			rand.Shuffle(d.RunEnv.TestInstanceCount-1, func(i, j int) {
				addrs[i], addrs[j] = addrs[j], addrs[i]
			})

			switch len(addrs) {
			case 0:
				return nil, errors.New("subscription closed")
			case 1, 2, 3:
				return addrs, nil
			default:
				return addrs[:3], nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

}

// Start advertising the service in the background.  Does not block.
// Subsequent calls to Start MUST be preceeded by a call to Close.
func (d *Discover) Start(s discover.Service) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	d.id = s.ID()
	as, err := s.Network().InterfaceListenAddresses()
	if err != nil {
		return err
	}

	if _, err = d.Client.Publish(ctx, topic, &peer.AddrInfo{
		ID:    d.id,
		Addrs: as,
	}); err != nil {
		return err
	}

	if _, err = d.Client.SignalEntry(ctx, stateDiscoverReady); err != nil {
		return err
	}

	return nil
}

// Close stops the active service advertisement.  Once called, Start can be called
// again.
func (d Discover) Close() error {
	return nil
}
