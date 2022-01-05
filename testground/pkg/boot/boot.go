package boot

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
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
	I int

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
			if (info == nil){
				println("info is nil")
				continue
			}
			if as = append(as, *info); len(as) == r.ClusterSize {
				return
			}
		}
	}

}


type MemoryDiscovery struct {
	Local       *peer.AddrInfo
	I int
	Hosts []host.Host

	Topo Topology
	once sync.Once

	Sa boot.StaticAddrs
}

func (m *MemoryDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (_ time.Duration, err error) {
	opts := discovery.Options{Ttl: peerstore.PermanentAddrTTL}
	if err = opts.Apply(opt...); err != nil {
		return
	}
	return opts.Ttl, err
}

func (m *MemoryDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	m.once.Do(func() {
		if m.Topo == nil {
			m.Topo = Ring{m.Local.ID}
		}
		m.Sa = append(boot.StaticAddrs{}, m.Sa...)  // make a copy
	})
	return m.Topo.GetNeighbors(m.Sa).FindPeers(ctx, ns, opt...)
}
