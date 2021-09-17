package boot

import (
	"context"
	"fmt"
	"time"
	"sync"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	peerstore "github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/testground/sdk-go/runtime"
	tsync "github.com/testground/sdk-go/sync"
	pboot "github.com/wetware/casm/pkg/boot"
)

const ringTopic = "ring"

type RedisDiscovery struct{
	env *runtime.RunEnv
	client tsync.Client 
	h host.Host
	t Topology
	peers pboot.StaticAddrs
	mu sync.Mutex
}
type DiscoverInfo struct{
}

func New(env *runtime.RunEnv, client tsync.Client, h host.Host, ns string) (*RedisDiscovery, error) {
	r := &RedisDiscovery{env:env, client: client, h: h, t: &Ring{}, peers: make(pboot.StaticAddrs, 0)}
	
	return r, nil
}

func (r *RedisDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (ttl time.Duration, err error) {
	// Subscribe to ring neighbors advertisements
	// NOP: just need to publish one time the peerID on New()
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if len(r.peers)==0{
		tch := make(chan *peer.AddrInfo)
		topic := fmt.Sprintf("%v.%v", ns, r.t.Name())
		subscribe(r.client, tch, topic)
		r.syncState(tsync.State("subscribed"))
		st := tsync.NewTopic(topic, &peer.AddrInfo{})
		r.client.Publish(context.Background(), st, host.InfoFromHost(r.h))

		for i:=0;i<r.env.TestInstanceCount;i++{
			r.peers = append(r.peers, *<-tch)
		}
		r.syncState(tsync.State("received"))
	}
	return peerstore.PermanentAddrTTL, nil
	
}

func (r *RedisDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.t.GetNeighbors(r.h.ID(), r.peers).FindPeers(ctx, ns, opt...)
}

func subscribe(client tsync.Client, tch chan *peer.AddrInfo, topic string) error{
	st := tsync.NewTopic(topic, &peer.AddrInfo{})
	_, err := client.Subscribe(context.Background(), st, tch)
	if err != nil {
		panic(err)
	}
	return nil
}

func (r *RedisDiscovery) syncState(state tsync.State) error{
	r.client.MustSignalEntry(context.Background(), state)
	err := <-r.client.MustBarrier(context.Background(), state, r.env.TestInstanceCount).C
	return err
}
