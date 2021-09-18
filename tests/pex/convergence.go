package pex

import (
	"context"
	"time"

	zaputil "github.com/lthibault/log/util/zap"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/pkg/boot"
)

// Run tests for PeX.
func RunConvergence(env *runtime.RunEnv, initCtx *run.InitContext) error {
	var (
		tick           = time.Millisecond * time.Duration(env.IntParam("tick")) // tick in miliseconds
		convTickAmount = env.IntParam("convTickAmount")
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h, err := libp2p.New(context.Background())
	if err != nil {
		return err
	}
	defer h.Close()
	sub, err := h.EventBus().Subscribe(new(pex.EvtViewUpdated))
	if err != nil {
		return err
	}
	go viewMetricsLoop(env, ctx, h, sub)

	d := &boot.RedisDiscovery{
		C:     initCtx.SyncClient,
		Local: host.InfoFromHost(h),
	}

	px, err := pex.New(ctx, h,
		pex.WithDiscovery(d),
		pex.WithTick(tick), // speed up the simulation
		pex.WithLogger(zaputil.Wrap(env.SLogger())))
	if err != nil {
		return err
	}

	// Advertise triggers a gossip round.  When a 'PeerExchange' instance
	// is provided to a 'PubSub' instance, this method will be called in
	// a loop with the interval specified by the TTL return value.
	env.RecordMessage("Entering loop")
	for i := 0; i < convTickAmount; i++ {
		env.RecordMessage("Advertising %d...", i)
		_, err = px.Advertise(ctx, ns)
		env.RecordMessage("Advertised %d", i)
		if err != nil {
			return err
		}
		time.Sleep(tick)
	}
	env.RecordMessage("Leaving loop")

	// TODO:  actual test starts here
	// Test 1: How fast does PeX converge on a uniform distribution of records?

	env.RecordSuccess()

	return nil
}
