package boot

import (
	"context"
	"errors"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
)

type RedisDiscovery struct{}

func New(env *runtime.RunEnv, initCtx *run.InitContext) (*RedisDiscovery, error) {

	return nil, errors.New("NOT IMPLEMENTED")
}

func (r *RedisDiscovery) Advertise(ctx context.Context, ns string, opt ...discovery.Option) (ttl time.Duration, err error) {
	panic("NOT IMPLEMENTED")
}

func (r *RedisDiscovery) FindPeers(ctx context.Context, ns string, opt ...discovery.Option) (<-chan peer.AddrInfo, error) {
	panic("NOT IMPLEMENTED")
}
