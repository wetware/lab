package pex

import (
	"context"
	"time"

	zaputil "github.com/lthibault/log/util/zap"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/discovery"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/pkg/boot"
)

const ns = "casm.lab.pex"

// Run tests for PeX.
func Run(env *runtime.RunEnv, initCtx *run.InitContext) error {
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
}
