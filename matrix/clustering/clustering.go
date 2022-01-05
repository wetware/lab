package main

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/wetware/casm/pkg/cluster"
	"github.com/wetware/casm/pkg/cluster/pulse"
	mx "github.com/wetware/matrix/pkg"
)

var (
	nodesAmount   = 100
	hs = make([]host.Host, nodesAmount)
	cs = make([]*cluster.Node, nodesAmount)
	tick = 100 * time.Millisecond
	waitFor = 10 * time.Second
)



func main(){
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sim := mx.New(ctx)
	println("Initializing cluster...")
	initCluster(ctx, sim)
	println("Initialized cluster!")

	timerWaitFor := time.NewTimer(waitFor)
	timerTick := time.NewTimer(tick)
	for{
		select {
		case <- timerWaitFor.C:
		case <- timerTick.C:
			println("N0 view length:", len(clusterView(cs[0])))
			timerTick.Reset(tick)
		}
	}
}

func clusterView(n *cluster.Node) (ps peer.IDSlice) {
	for it := n.View().Iter(); it.Record() != nil; it.Next() {
		ps = append(ps, it.Record().Peer())
	}

	return
}

type MyMeta struct {}

func (meta *MyMeta) Prepare(hb pulse.Heartbeat){
	hb.Meta().SetText("This is my meta!")
}

func initCluster(ctx context.Context, sim mx.Simulation){
	var wg sync.WaitGroup

	// init hosts
	for i := 0; i < nodesAmount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			hs[id] = sim.MustHost(ctx)
		}(i)
	}
	wg.Wait()

	// init casm cluster nodes
	for i := 0; i < nodesAmount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// decide neighbors to bootstrap pubsub
			id1, id2 := id-1, id+1
			if id1 < 0 {
				id1 = nodesAmount - 1
			}
			if id2 >= nodesAmount {
				id2 = 0
			}

			// init pubsub + cluster node
			ps, err := pubsub.NewGossipSub(ctx, hs[id],
				pubsub.WithDirectPeers([]peer.AddrInfo{*host.InfoFromHost(hs[id1]), *host.InfoFromHost(hs[id2])}))
			if err != nil {
				panic(err)
			}
			cs[id], err = cluster.New(ctx, ps, cluster.WithMeta(&MyMeta{}))
			if err != nil {
				panic(err)
			}

		}(i)
	}

	wg.Wait()
}