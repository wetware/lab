package testutil

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

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
func (d *Discover) DiscoverPeers(ctx context.Context, opt ...discover.Option) (<-chan peer.AddrInfo, error) {
	var p discover.Param
	if err := p.Apply(opt); err != nil {
		return nil, err
	}

	b, err := d.Client.Barrier(ctx, stateDiscoverReady, d.RunEnv.TestInstanceCount)
	if err != nil {
		return nil, err
	}

	if err = <-b.C; err != nil {
		return nil, err
	}

	// TODO:  open issue inquiring about purpose of sub (esp. sub.Done)
	ch := make(chan peer.AddrInfo, 1)
	if _, err = d.Client.Subscribe(ctx, topic, ch); err != nil {
		return nil, err
	}

	out := make(chan peer.AddrInfo, 1)
	go func() {
		defer close(out)

		remaining := p.Limit
		for info := range ch {
			if info.ID == d.id {
				continue
			}

			select {
			case out <- info:
				if p.Limit > 0 {
					if remaining--; remaining == 0 {
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
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
