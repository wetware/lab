package pex

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/wetware/casm/pkg/pex"
	"github.com/testground/sdk-go/runtime"
)

const ns = "casm.lab.pex"

func viewMetricsLoop(env *runtime.RunEnv, ctx context.Context, h host.Host, sub event.Subscription) error {
	tick := 1
	for {
		select {
		case v := <-sub.Out():
			view := pex.View(v.(pex.EvtViewUpdated))
			viewString := ""
			for _, pr := range view{
				viewString = fmt.Sprintf("%v:%v", viewString, pr.PeerID)
			}
			name := fmt.Sprintf("view,peer=%v,view_ids=%v,tick=%v", h.ID(), viewString, tick)
			env.R().RecordPoint(name, 0)  // The value is not used
			tick += 1
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}