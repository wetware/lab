package boot

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	pexboot "github.com/wetware/casm/pkg/boot"
)

const ringTopic = "ring"

type RedisDiscovery struct{
	env *runtime.RunEnv
	client sync.Client 
	h host.Host
	t Topology
	neighbors pexboot.StaticAddrs
	found bool
}
type DiscoverInfo struct{
}

func New(env *runtime.RunEnv, client sync.Client, h host.Host, ns string) (*RedisDiscovery, error) {
	r := &RedisDiscovery{env, client, h, &Ring{}, make(pexboot.StaticAddrs, 0,2), false}
	
	tch := make(chan *peer.AddrInfo)
	topic := fmt.Sprintf("%v.%v", ns, r.t.Name())
	subscribe(client, tch, topic)
	r.syncState(sync.State("subscribed"))
	st := sync.NewTopic(topic, &peer.AddrInfo{})
	r.client.Publish(context.Background(), st, host.InfoFromHost(r.h))

	boot := make(pexboot.StaticAddrs, 0)
	for i:=0;i<env.TestInstanceCount;i++{
		p := *<-tch
		boot = append(boot, p)
	}
	r.syncState(sync.State("received"))
	r.neighbors = r.t.GetNeighbors(h.ID(), boot)
	return r, nil
}

func (r *RedisDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (ttl time.Duration, err error) {
	// Subscribe to ring neighbors advertisements
	// NOP: just need to publish one time the peerID on New()
	
	return peerstore.PermanentAddrTTL, nil
	
}

func (r *RedisDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	ch := make(chan peer.AddrInfo, 2)
	for _, n := range r.neighbors{
		ch <- n
	}
	close(ch)
	return ch, nil
}

func subscribe(client sync.Client, tch chan *peer.AddrInfo, topic string) error{
	st := sync.NewTopic(topic, &peer.AddrInfo{})
	_, err := client.Subscribe(context.Background(), st, tch)
	if err != nil {
		panic(err)
	}
	return nil
}

func (r *RedisDiscovery) syncState(state sync.State) error{
	r.client.MustSignalEntry(context.Background(), state)
	err := <-r.client.MustBarrier(context.Background(), state, r.env.TestInstanceCount).C
	return err
}
