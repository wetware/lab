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
)

const ringTopic = "ring"

type RedisDiscovery struct{
	env *runtime.RunEnv
	client sync.Client 
	h host.Host
	seq int64
	tch chan *peer.AddrInfo
}
type DiscoverInfo struct{
}

func New(env *runtime.RunEnv, client sync.Client, h host.Host, seq int64) (*RedisDiscovery, error) {
	r := &RedisDiscovery{env, client, h, seq, make(chan *peer.AddrInfo)}
	return r, nil
}

func (r *RedisDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (ttl time.Duration, err error) {
	// Subscribe to ring neighbors advertisements
	n1, n2 := ringNeighbors(r.env, r.seq)
	t1 := fmt.Sprintf("%v.%v.%v", ns, ringTopic, n1)
	t2 := fmt.Sprintf("%v.%v.%v", ns, ringTopic, n2)
	r.subscribe(t1)
	r.subscribe(t2)
	err = r.syncState(sync.State("subscribed"));
	// Advertise
	topic := fmt.Sprintf("%v.%v.%v", ns, ringTopic, r.seq)
	st := sync.NewTopic(topic, &peer.AddrInfo{})
	r.client.Publish(ctx, st, host.InfoFromHost(r.h))
	return peerstore.PermanentAddrTTL, nil
	
}

func (r *RedisDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	ch := make(chan peer.AddrInfo, 2)
	ch <- *<-r.tch  // ring neighbor 1
	ch <- *<-r.tch // ring neighbor 2
	close(ch)
	return ch, nil
}

func (r *RedisDiscovery) subscribe(topic string) error{
	st := sync.NewTopic(topic, &peer.AddrInfo{})
	_, err := r.client.Subscribe(context.Background(), st, r.tch)
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

func ringNeighbors(env *runtime.RunEnv, seq int64) (n1, n2 int64){
	n1 = (seq + 1)%int64(env.TestInstanceCount)
	n2 = seq -1
	if n2 < 0{
		return n1, int64(env.TestInstanceCount)-1
	}
	return 
}