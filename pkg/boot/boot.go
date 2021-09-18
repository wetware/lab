package boot

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	tsync "github.com/testground/sdk-go/sync"
	"github.com/wetware/casm/pkg/boot"
)

type RedisDiscovery struct {
	C     tsync.Client
	Local *peer.AddrInfo

	once sync.Once
	Topo Topology

	as atomic.Value
}

func (r *RedisDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (_ time.Duration, err error) {
	opts := discovery.Options{Ttl: peerstore.PermanentAddrTTL}
	if err = opts.Apply(opt...); err != nil {
		return
	}

	if r.as.Load() == nil {
		var as boot.StaticAddrs
		if as, err = r.syncRedis(ctx, ns); err == nil {
			r.as.CompareAndSwap(nil, as) // concurrent call may have already stored results
		}
	}

	return opts.Ttl, err

}

func (r *RedisDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	r.once.Do(func() {
		if r.Topo == nil {
			r.Topo = Ring{r.Local.ID}
		}
	})

	var as boot.StaticAddrs
	if v := r.as.Load(); v != nil {
		as = v.(boot.StaticAddrs)
	}

	return r.Topo.GetNeighbors(as).FindPeers(ctx, ns, opt...)
}

func (r *RedisDiscovery) syncRedis(ctx context.Context, ns string) (as boot.StaticAddrs, err error) {
	var (
		ch  = make(chan *peer.AddrInfo, 1)
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
			as = append(as, *info)
		}
	}
}
