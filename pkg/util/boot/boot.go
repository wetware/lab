package testutil

// import (
// 	"context"
// 	"time"

// 	"github.com/libp2p/go-libp2p-core/peer"

// 	"github.com/testground/sdk-go/runtime"
// 	"github.com/testground/sdk-go/sync"

// 	"github.com/wetware/ww/pkg/boot"
// )

// var (
// 	topic       = sync.NewTopic("boot", new(peer.AddrInfo))
// 	stateBooted = sync.State("booted")
// )

// // Boot implements bootstrappig over github.com/testground/sdk-go/sync.
// // It does not close the underlying sync.Client.
// type Boot struct {
// 	RunEnv *runtime.RunEnv
// 	Client sync.Client

// 	id peer.ID
// }

// // DiscoverPeers over Testground sync service.
// func (b *Boot) DiscoverPeers(ctx context.Context, opt ...boot.Option) (<-chan peer.AddrInfo, error) {
// 	var p boot.Param
// 	if err := p.Apply(opt); err != nil {
// 		return nil, err
// 	}

// 	bar, err := b.Client.Barrier(ctx, stateBooted, b.RunEnv.TestInstanceCount)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if err = <-bar.C; err != nil {
// 		return nil, err
// 	}

// 	// TODO:  open issue inquiring about purpose of sub (esp. sub.Done)
// 	ch := make(chan peer.AddrInfo, 1)
// 	if _, err = b.Client.Subscribe(ctx, topic, ch); err != nil {
// 		return nil, err
// 	}

// 	out := make(chan peer.AddrInfo, 1)
// 	go func() {
// 		defer close(out)

// 		remaining := p.Limit
// 		for info := range ch {
// 			if info.ID == b.id {
// 				continue
// 			}

// 			select {
// 			case out <- info:
// 				if p.Limit > 0 {
// 					if remaining--; remaining == 0 {
// 						return
// 					}
// 				}
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}()

// 	return out, nil
// }

// // Start advertising the service in the background.  Does not block.
// // Subsequent calls to Start MUST be preceeded by a call to Close.
// func (b *Boot) Start(s boot.Service) error {
// 	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
// 	defer cancel()

// 	b.id = s.ID()
// 	as, err := s.Network().InterfaceListenAddresses()
// 	if err != nil {
// 		return err
// 	}

// 	if _, err = b.Client.Publish(ctx, topic, &peer.AddrInfo{
// 		ID:    b.id,
// 		Addrs: as,
// 	}); err != nil {
// 		return err
// 	}

// 	if _, err = b.Client.SignalEntry(ctx, stateBooted); err != nil {
// 		return err
// 	}

// 	return nil
// }

// // Close stops the active service advertisement.  Once called, Start can be called
// // again.
// func (b Boot) Close() error {
// 	return nil
// }
