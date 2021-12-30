package pex

import (
	"context"
	"github.com/libp2p/go-libp2p-core/host"
	zaputil "github.com/lthibault/log/util/zap"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	boot2 "github.com/wetware/casm/pkg/boot"
	"github.com/wetware/casm/pkg/pex"
	"github.com/wetware/lab/pkg/boot"
	mx "github.com/wetware/matrix/pkg"
	//"github.com/libp2p/go-libp2p-core/network"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"github.com/influxdata/influxdb-client-go/v2"
)

func RunConvergenceMatrix(env *runtime.RunEnv, initCtx *run.InitContext) error {

	var (
		tick        = time.Millisecond * time.Duration(env.IntParam("tick")) // tick in miliseconds
		tickAmount  = env.IntParam("tickAmount")
		nodesAmount = env.IntParam("nodesAmount")
		c           = env.IntParam("c")
		s           = env.IntParam("s")
		r           = env.IntParam("r")
		d           = env.FloatParam("d")
		partitionTick           = env.IntParam("partitionTick")
	)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	ctx, cancel := context.WithCancel(context.Background())
	//ctx = network.WithDialPeerTimeout(ctx, time.Millisecond*100)
	defer cancel()

	// INflux
	client := influxdb2.NewClientWithOptions("http://localhost:8087", "my-token",
		influxdb2.DefaultOptions().SetBatchSize(20))
	// Get non-blocking write client
	writeAPI := client.WriteAPI("my-org","testground")
	//

	sim := mx.New(ctx)

	hs := make([]host.Host, nodesAmount)
	pxs := make([]*pex.PeerExchange, nodesAmount)
	sa := make(boot2.StaticAddrs, nodesAmount)
	gossip := pex.GossipParams{c, s, r, d}

	env.RecordMessage("Initializing nodes...")
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
				env.RecordFailure(err)
			}
			go viewMetricsLoop(ctx, writeAPI, h, sub, &gossip)
			d := &boot.MemoryDiscovery{
				Local: host.InfoFromHost(h),
				Sa:    sa,
				I: id,
			}

			px, err := pex.New(ctx, h,
				pex.WithGossipParams(gossip),
				pex.WithDiscovery(d),
				pex.WithTick(tick), // speed up the simulation
				pex.WithLogger(zaputil.Wrap(env.SLogger())))
			if err != nil {
				env.RecordFailure(err)
			}
			pxs[id] = px
		}(i)
	}
	wg.Wait()
	env.RecordMessage("Initialized nodes...")
	errorsAmount := int64(0)
	for t := 0; t < tickAmount; t++ {
		errorsAmount = 0
		if t == partitionTick{
			rand.Shuffle(len(pxs), func(i, j int) { pxs[i], pxs[j], hs[i], hs[j] = pxs[j], pxs[i], hs[j], hs[i] })
			evict := pxs[:len(pxs)/2]
			pxs = pxs[len(evict):]
			for i, px := range evict{
				if err := px.Close(); err != nil {
					return err
				}
				if err := hs[i].Close(); err != nil {
					return err
				}
			}
		}
		env.RecordMessage("Tick %v/%v\n", t+1, tickAmount)
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
					env.RecordMessage(err.Error())
					atomic.AddInt64(&errorsAmount, 1)
				}
			}(i, p)
		}
		wg.Wait()
		for evtAmount < (int64(len(pxs)) - errorsAmount)*2  {}
		atomic.AddInt64(&evtAmount, -((int64(len(pxs)) - errorsAmount)*2))
		atomic.AddInt64(&metricTick, 1)
	}
	env.RecordSuccess()

	return nil
}
