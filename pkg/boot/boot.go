package boot

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	tsync "github.com/testground/sdk-go/sync"
	"github.com/wetware/casm/pkg/boot"
)

type RedisDiscovery struct {
	ClusterSize int
	C           tsync.Client
	Local       *peer.AddrInfo

	once sync.Once
	Topo Topology

	mu sync.RWMutex
	sa boot.StaticAddrs
}

func (r *RedisDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (_ time.Duration, err error) {
	opts := discovery.Options{Ttl: peerstore.PermanentAddrTTL}
	if err = opts.Apply(opt...); err != nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sa == nil {
		r.sa, err = r.syncRedis(ctx, ns)
	}

	return opts.Ttl, err

}

func (r *RedisDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	r.once.Do(func() {
		if r.Topo == nil {
			r.Topo = Ring{r.Local.ID}
		}
	})

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.Topo.GetNeighbors(r.sa).FindPeers(ctx, ns, opt...)
}

func (r *RedisDiscovery) syncRedis(ctx context.Context, ns string) (as boot.StaticAddrs, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		ch  = make(chan *peer.AddrInfo, r.ClusterSize)
		sub *tsync.Subscription
	)

	if _, sub, err = r.C.PublishSubscribe(ctx,
		tsync.NewTopic(ns, new(peer.AddrInfo)),
		r.Local, ch,
	); err != nil {
		return
	}

	for {
		select {
		case err = <-sub.Done():
			return
		case info := <-ch:
			if as = append(as, *info); len(as) == r.ClusterSize {
				return
			}
		}
	}
}
