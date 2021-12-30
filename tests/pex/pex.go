package pex

import (
	"context"
	"fmt"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/wetware/casm/pkg/pex"
	"golang.org/x/sync/semaphore"
	"sync/atomic"
)

const ns = "casm.lab.pex"

var (
	metricTick int64 = 1
	evtAmount int64 = 0
	sem              = semaphore.NewWeighted(int64(50))
	shortID          = make(map[peer.ID]int, 0)
)


func viewMetricsLoop(ctx context.Context, writeAPI api.WriteAPI, h host.Host, sub event.Subscription, gossip *pex.GossipParams) error {

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
				"C=%v,S=%v,R=%v,D=%v value=0", shortID[h.ID()], viewString, 0, metricTick,
				gossip.C, gossip.S, gossip.R, gossip.D)
			writeAPI.WriteRecord(name)
			sem.Release(1)
			atomic.AddInt64(&evtAmount, 1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
