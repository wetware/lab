package pex

import (
	"context"
	"time"

	zaputil "github.com/lthibault/log/util/zap"
	"go.uber.org/zap"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/pkg/boot"
	tsync "github.com/testground/sdk-go/sync"
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
	go viewMetricsLoop(ctx, env, h, sub)

	d := &boot.RedisDiscovery{
		ClusterSize: env.TestInstanceCount,
		C:           initCtx.SyncClient,
		Local:       host.InfoFromHost(h),
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
	for i := 0; i < convTickAmount; i++ {
		ttl, err := px.Advertise(ctx, ns)
		if err != nil {
			return err
		}

		env.SLogger().
			With(zap.Duration("ttl", ttl)).
			Debug("call to advertise succeeded")
	}
	
	initCtx.SyncClient.MustSignalAndWait(ctx, tsync.State("finished"), env.TestInstanceCount)
	// TODO:  actual test starts here
	// Test 1: How fast does PeX converge on a uniform distribution of records?

	env.RecordSuccess()
	return nil
}
