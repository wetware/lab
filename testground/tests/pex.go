package tests

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	zaputil "github.com/lthibault/log/util/zap"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	tsync "github.com/testground/sdk-go/sync"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/testground/pkg/boot"
)

const ns = "casm.lab.pex"

var (
	metricTick int64 = 1
	evtAmount  int64 = 0
	sem              = semaphore.NewWeighted(int64(100))
	shortID          = make(map[peer.ID]int, 0)
)

func recordLocalRecordUpdates(ctx context.Context, env *runtime.RunEnv, h host.Host, sub event.Subscription, gossip *pex.Gossip) error {

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
				"C=%v,S=%v,P=%v,D=%v,run=%v", shortID[h.ID()], viewString, 0, metricTick,
				gossip.C, gossip.S, gossip.P, gossip.D, env.TestGroupID)
			env.D().RecordPoint(name, 0)
			sem.Release(1)
			atomic.AddInt64(&evtAmount, 1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Run tests for PeX.
func RunPex(env *runtime.RunEnv, initCtx *run.InitContext) error {
	var (
		tick           = time.Millisecond * time.Duration(env.IntParam("tick")) // tick in miliseconds
		tickAmount = env.IntParam("tickAmount")
		c = env.IntParam("c")
		s = env.IntParam("s")
		p = env.IntParam("p")
		d = env.FloatParam("d")
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
	gossip := pex.Gossip{c, s, p, d}
	go recordLocalRecordUpdates(ctx, env, h, sub, &gossip)

	disc := &boot.RedisDiscovery{
		ClusterSize: env.TestInstanceCount,
		C:           initCtx.SyncClient,
		Local:       host.InfoFromHost(h),
	}
	

	px, err := pex.New(ctx, h,
		pex.WithGossip(func (ns string) pex.Gossip {return gossip}),
		pex.WithDiscovery(disc),
		pex.WithTick(func (ns string) time.Duration {return tick}), // speed up the simulation
		pex.WithLogger(zaputil.Wrap(env.SLogger())))
	if err != nil {
		return err
	}

	// Advertise triggers a gossip round.  When a 'PeerExchange' instance
	// is provided to a 'PubSub' instance, this method will be called in
	// a loop with the interval specified by the TTL return value.
	initCtx.SyncClient.MustSignalAndWait(ctx, tsync.State("initialized"), env.TestInstanceCount)
	for i := 0; i < tickAmount; i++ {
		if initCtx.GlobalSeq == 1{
			fmt.Printf("Tick %v/%v\n", i+1, tickAmount)
		}
		ttl, err := px.Advertise(ctx, ns)
		if err != nil && !strings.Contains(err.Error(), "stream reset") &&
			!strings.Contains(err.Error(), "failed to dial") &&
			!strings.Contains(err.Error(), "i/o deadline reached"){
			return err
		}
		if err != nil{
			env.RecordMessage(err.Error())
		}
		env.SLogger().
			With(zap.Duration("ttl", ttl)).
			Debug("call to advertise succeeded")
		initCtx.SyncClient.MustSignalAndWait(ctx, tsync.State(fmt.Sprintf("advertised %v", i)), env.TestInstanceCount)
		atomic.AddInt64(&metricTick, 1)
		initCtx.SyncClient.MustSignalAndWait(ctx, tsync.State(fmt.Sprintf("ticked %v", i)), env.TestInstanceCount)

	}
	env.RecordSuccess()
	initCtx.SyncClient.MustSignalAndWait(ctx, tsync.State("finished"), env.TestInstanceCount)

	return nil
}

func RunDnsTest(env *runtime.RunEnv, initCtx *run.InitContext) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if (initCtx.GlobalSeq!=1){
		env.RecordSuccess()
		initCtx.SyncClient.MustSignalAndWait(ctx, tsync.State("finished"), env.TestInstanceCount)
	} else{
		for{}
	}
	return nil
}