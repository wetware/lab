package pex

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/testground/sdk-go/runtime"
	"github.com/wetware/casm/pkg/pex"
)

const ns = "casm.lab.pex"
var metricTick int64 = 1

func viewMetricsLoop(ctx context.Context, env *runtime.RunEnv, h host.Host, sub event.Subscription) error {

	for {
		select {
		case v := <-sub.Out():
			view := []*pex.GossipRecord(v.(pex.EvtViewUpdated))
			viewString := ""
			for _, pr := range view {
				viewString = fmt.Sprintf("%v-%v", viewString, pr.PeerID)
			}
			name := fmt.Sprintf("view,node=%v,records=%v,tick=%v", h.ID(), viewString, metricTick)
			env.D().RecordPoint(name, 0) // The value is not used
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
