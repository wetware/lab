package lab

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/wetware/ww/pkg/boot"
)

var (
	topic       = sync.NewTopic("boot", new(peer.AddrInfo))
	stateBooted = sync.State("booted")
)

// Bootstrapper implements bootstrappig over github.com/testground/sdk-go/sync.
// It does not close the underlying sync.Client.
type Bootstrapper struct {
	RunEnv     *runtime.RunEnv
	SyncClient sync.Client

	id peer.ID
}

// Loggable representation of the bootstrapper
func (b Bootstrapper) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"service": "lab.boot",
		"peer_id": b.id,
	}
}

// DiscoverPeers over Testground sync service.
func (b *Bootstrapper) DiscoverPeers(ctx context.Context, opt ...boot.Option) (<-chan peer.AddrInfo, error) {
	var p boot.Param
	if err := p.Apply(opt); err != nil {
		return nil, err
	}

	bar, err := b.SyncClient.Barrier(ctx, stateBooted, b.RunEnv.TestInstanceCount)
	if err != nil {
		return nil, err
	}

	if err = <-bar.C; err != nil {
		return nil, err
	}

	// TODO:  open issue inquiring about purpose of sub (esp. sub.Done)
	ch := make(chan peer.AddrInfo, 1)
	if _, err = b.SyncClient.Subscribe(ctx, topic, ch); err != nil {
		return nil, err
	}

	out := make(chan peer.AddrInfo, 1)
	go func() {
		defer close(out)

		remaining := p.Limit
		for info := range ch {
			if info.ID == b.id {
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

// Signal the host's presence on the network.
func (b *Bootstrapper) Signal(ctx context.Context, h host.Host) error {
	b.id = h.ID()
	as, err := h.Network().InterfaceListenAddresses()
	if err != nil {
		return err
	}

	if _, err = b.SyncClient.Publish(ctx, topic, &peer.AddrInfo{
		ID:    b.id,
		Addrs: as,
	}); err != nil {
		return err
	}

	if _, err = b.SyncClient.SignalEntry(ctx, stateBooted); err != nil {
		return err
	}

	return nil
}

// Stop the active service advertisement the beacon
func (b Bootstrapper) Stop(context.Context) error {
	return nil
}
