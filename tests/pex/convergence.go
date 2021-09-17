package pex

import (
	"context"
	"time"

	zaputil "github.com/lthibault/log/util/zap"

	"github.com/libp2p/go-libp2p"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
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

	// instantiate a sync service client, binding it to the RunEnv.
	client := initCtx.SyncClient

	h, err := libp2p.New(context.Background())
	if err != nil {
		return err
	}
	defer h.Close()
	sub, err := h.EventBus().Subscribe(new(pex.EvtViewUpdated))
	if err !=nil{
		return err
	}
	go viewMetricsLoop(env, ctx, h, sub)

	d, err := boot.New(env, client, h, ns)
	if err != nil {
		return err
	}

	px, err := pex.New(ctx, h,
		pex.WithDiscovery(d),		
		pex.WithTick(tick),    // speed up the simulation
		pex.WithLogger(zaputil.Wrap(env.SLogger())))
	if err != nil {
		return err
	}

	
	
	// Advertise triggers a gossip round.  When a 'PeerExchange' instance
	// is provided to a 'PubSub' instance, this method will be called in
	// a loop with the interval specified by the TTL return value.
	env.RecordMessage("Entering loop")
	for i:=0; i<convTickAmount; i++{
		env.RecordMessage("Advertising %d...", i)
		_, err = px.Advertise(ctx, ns)
		env.RecordMessage("Advertised %d", i)
		if err != nil {
			return err
		}
		env.RecordMessage("Sleeping %d...", i)
		time.Sleep(tick)
		env.RecordMessage("Awaken %d", i)
	}
	env.RecordMessage("Leaving loop")
	

	// TODO:  actual test starts here
	// Test 1: How fast does PeX converge on a uniform distribution of records?
	
	env.RecordSuccess()

	return nil
}