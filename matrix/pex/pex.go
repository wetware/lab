package main

import (
	"context"
	"fmt"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/thanhpk/randstr"
	"github.com/wetware/casm/pkg/boot"
	"github.com/wetware/casm/pkg/pex"
	mx "github.com/wetware/matrix/pkg"
	"golang.org/x/sync/semaphore"
	"sort"
	"time"

	"math/rand"
	"sync"
	"sync/atomic"
)

const ns = "casm.lab.pex"

var run = randstr.Hex(16)

type Topology interface {
	GetNeighbors(boot boot.StaticAddrs) boot.StaticAddrs
}

type Ring struct{ ID peer.ID }

func (r Ring) GetNeighbors(sa boot.StaticAddrs) boot.StaticAddrs {
	if len(sa) < 2 {
		return sa
	}

	sort.Sort(sa)

	// Defensively range over the values instead of a 'while' loop,
	// in case the target ID is not contained in the slice.  This
	// should never happen, but you never know...
	for range sa {
		if sa[0].ID == r.ID {
			break
		}

		sa = rotateLeft(sa)
	}

	// use filter instead of indexing in case len(as) == 0
	return sa.Filter(func(info peer.AddrInfo) bool {
		return info.ID != r.ID
	})[:2]
}

func rotateLeft(as boot.StaticAddrs) boot.StaticAddrs {
	return append(as[1:], as[0]) // always len(as) > 1
}

type MemoryDiscovery struct {
	Local *peer.AddrInfo
	I     int
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
		m.Sa = append(boot.StaticAddrs{}, m.Sa...) // make a copy
	})
	return m.Topo.GetNeighbors(m.Sa).FindPeers(ctx, ns, opt...)
}

var (
	metricTick int64 = 1
	evtAmount  int64 = 0
	sem              = semaphore.NewWeighted(int64(100))
	shortID          = make(map[peer.ID]int, 0)
)

func viewMetricsLoop(ctx context.Context, writeAPI api.WriteAPIBlocking, h host.Host, sub event.Subscription, gossip *pex.Gossip) error {

	for {
		select {
		case v := <-sub.Out():
			if err := sem.Acquire(ctx, 1); err != nil {
				break
			}
			view := []*pex.GossipRecord(v.(pex.EvtViewUpdated))
			viewString := ""
			for _, pr := range view {
				viewString = fmt.Sprintf("%v-%v", viewString, shortID[pr.PeerID])
			}

			name := fmt.Sprintf("view,node=%v,records=%v,cluster=%v,tick=%v,"+
				"C=%v,S=%v,P=%v,D=%v,run=%v value=0", shortID[h.ID()], viewString, 0, metricTick,
				gossip.C, gossip.S, gossip.P, gossip.D, run)
			err := writeAPI.WriteRecord(ctx, name)
			if err != nil {
				println("Influx error:", err)
			}
			sem.Release(1)
			atomic.AddInt64(&evtAmount, 1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func main() {

	var (
		tickAmount    = 30
		nodesAmount   = 1000
		c             = 30
		s             = 10
		p             = 5
		d             = 0.005
		partitionTick = -1
	)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := influxdb2.NewClient("http://localhost:8086", "my-token")
	// Use blocking write client for writes to desired bucket
	writeAPI := client.WriteAPIBlocking("my-org", "testground")

	sim := mx.New(ctx)

	hs := make([]host.Host, nodesAmount)
	pxs := make([]*pex.PeerExchange, nodesAmount)
	sa := make(boot.StaticAddrs, nodesAmount)
	gossip := pex.Gossip{c, s, p, d}
	println("Run: ", run)
	println("Initializing nodes...")
	for i := 0; i < nodesAmount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h := sim.MustHost(ctx)
			hs[id] = h
			mu.Lock()
			shortID[h.ID()] = id
			sa[id] = *host.InfoFromHost(h)
			mu.Unlock()
			sub, err := h.EventBus().Subscribe(new(pex.EvtViewUpdated))
			if err != nil {
				panic(err)
			}
			go viewMetricsLoop(ctx, writeAPI, h, sub, &gossip)
			d := &MemoryDiscovery{
				Local: host.InfoFromHost(h),
				Sa:    sa,
				I:     id,
			}

			px, err := pex.New(ctx, h,
				pex.WithGossip(func(ns string) pex.Gossip { return gossip }),
				pex.WithDiscovery(d))
			if err != nil {
				panic(err)
			}
			pxs[id] = px
		}(i)
	}
	wg.Wait()
	println("Initialized nodes...")
	errorsAmount := int64(0)
	for t := 0; t < tickAmount; t++ {
		errorsAmount = 0
		if t == partitionTick {
			rand.Shuffle(len(pxs), func(i, j int) { pxs[i], pxs[j], hs[i], hs[j] = pxs[j], pxs[i], hs[j], hs[i] })
			evict := pxs[:len(pxs)/2]
			pxs = pxs[len(evict):]
			for i, px := range evict {
				if err := px.Close(); err != nil {
					panic(err)
				}
				if err := hs[i].Close(); err != nil {
					panic(err)
				}
			}
		}
		fmt.Printf("Tick %v/%v\n", t+1, tickAmount)
		for i, p := range pxs {
			wg.Add(1)
			go func(id int, px *pex.PeerExchange) {
				var err error
				if t == 0 {
					if id == nodesAmount-1 {
						err = px.Bootstrap(ctx, ns, *host.InfoFromHost(hs[0]))
					} else {
						err = px.Bootstrap(ctx, ns, *host.InfoFromHost(hs[id+1]))
					}
				} else {
					_, err = px.Advertise(ctx, ns)
				}
				wg.Done()
				if err != nil {
					println("Advertise error:", err.Error())
					atomic.AddInt64(&errorsAmount, 1)
				}
			}(i, p)
		}
		wg.Wait()
		for evtAmount < (int64(len(pxs))-errorsAmount)*2 {
		}
		atomic.AddInt64(&evtAmount, -((int64(len(pxs)) - errorsAmount) * 2))
		atomic.AddInt64(&metricTick, 1)
	}
	println("SUCCESS")
	println("Run:", run)

}
