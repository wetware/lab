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
			view := []*pex.GossipRecord(v.(pex.EvtViewUpdated))
			viewString := ""
			for _, pr := range view{
				viewString = fmt.Sprintf("%v-%v", viewString, pr.PeerID)
			}
			name := fmt.Sprintf("view,peer=%v,records=%v,tick=%v", h.ID(), viewString, tick)
			env.D().RecordPoint(name, 0)  // The value is not used

			tick += 1
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
// Run tests for PeX.
/* func Run(env *runtime.RunEnv, initCtx *run.InitContext) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := boot.New(env, initCtx)
	if err != nil {
		return err
	}

	// FIXME:  we need to explicitly advertise until PeX takes care
	//         of this for us.
	_, err = d.Advertise(ctx, ns, discovery.TTL(time.Hour))
	if err != nil {
		return err
	}

	h, err := libp2p.New(context.Background())
	if err != nil {
		return err
	}
	defer h.Close()

	px, err := pex.New(h,
		pex.WithDiscovery(d),
		pex.WithTick(time.Millisecond*100), // speed up the simulation
		pex.WithLogger(zaputil.Wrap(env.SLogger())))
	if err != nil {
		return err
	}

	// Advertise triggers a gossip round.  When a 'PeerExchange' instance
	// is provided to a 'PubSub' instance, this method will be called in
	// a loop with the interval specified by the TTL return value.
	_, err = px.Advertise(ctx, ns)
	if err != nil {
		return err
	}
	// TODO:  actual test starts here

	return nil
} */
