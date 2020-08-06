package lab

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/testground/sdk-go/sync"
	"github.com/wetware/ww/pkg/boot"
	"github.com/wetware/ww/pkg/host"
	"golang.org/x/sync/errgroup"
)

// // BuildGraph with the specified topology.
// func BuildGraph(ctx context.Context, c sync.Client, tf TopologyFactory, h host.Host, n int) error {
// 	return GraphBuilder{
// 		N:      n,
// 		TF:     tf,
// 		Client: c,
// 	}.Build(ctx, h)
// }

// // GraphStrategy returns a boot.Strategy for connecting to the specified graph.
// // The Topology is used to select which peers should be returned by DiscoverPeers.
// func GraphStrategy(c sync.Client, t Topology, n int) boot.Strategy {
// 	return GraphJoiner{
// 		t: t,
// 		c: c,
// 		n: n,
// 	}
// }

// GraphBuilder .
type GraphBuilder struct {
	N      int             // how many hosts are in the cluster?
	TF     TopologyFactory // for use with Build
	Client sync.Client
	// l  PublishListener
}

// Build graph.
func (b GraphBuilder) Build(ctx context.Context, h host.Host) error {
	hp, err := b.listen(ctx, h)
	if err != nil {
		return err
	}

	hs, err := hp.Hosts(ctx)
	if err != nil {
		return errors.Wrap(err, "build host set")
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, info := range b.TF.NewTopology(h.ID()).Neighbors(hs) {
		g.Go(join(ctx, h, info))
	}

	return g.Wait()
}

func (b GraphBuilder) listen(ctx context.Context, h host.Host) (*hostSetFactory, error) {
	info, err := addrInfo(h)
	if err != nil {
		return nil, err
	}

	// publish local peer's AddrInfo and await full set of peers.
	ch := make(chan peer.AddrInfo, 1)
	_, sub, err := b.Client.PublishSubscribe(ctx, topic, info, ch)
	if err != nil {
		return nil, err
	}

	return &hostSetFactory{
		n:   b.N,
		sub: sub,
		ch:  ch,
	}, nil
}

// GraphJoiner is a boot.Strategy for joining graph topologies.
// It is generally used by ww's client.Client
type GraphJoiner struct {
	N      int
	T      Topology
	Client sync.Client
}

// DiscoverPeers to whom we should connect.
func (g GraphJoiner) DiscoverPeers(ctx context.Context, opt ...boot.Option) (<-chan peer.AddrInfo, error) {
	var p boot.Param
	if err := p.Apply(opt); err != nil {
		return nil, err
	}

	hp, err := g.listen(ctx)
	if err != nil {
		return nil, err
	}

	hs, err := hp.Hosts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "build host set")
	}

	var ch chan peer.AddrInfo
	var ns = g.T.Neighbors(hs)
	if p.Limit == 0 {
		ch = make(chan peer.AddrInfo, len(ns))
	} else {
		ch = make(chan peer.AddrInfo, min(len(ns), p.Limit))
	}

	for _, info := range ns {
		ch <- info
	}

	return ch, nil
}

// Loggable representation of the GraphJoiner
func (g GraphJoiner) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"service": "lab.GraphJoiner",
	}
}

func (g GraphJoiner) listen(ctx context.Context) (*hostSetFactory, error) {
	ch := make(chan peer.AddrInfo, 1)
	sub, err := g.Client.Subscribe(ctx, topic, ch)
	if err != nil {
		return nil, err
	}

	return &hostSetFactory{
		n:   g.N,
		sub: sub,
		ch:  ch,
	}, nil
}

// // BuildGraph with the specified technology.
// func BuildGraph(ctx context.Context, c sync.Client, t Topology, h host.Host, n int) error {

// 	info, err := addrInfo(h)
// 	if err != nil {
// 		return err
// 	}

// 	// publish local peer's AddrInfo and await full set of peers.
// 	ch := make(chan peer.AddrInfo, 1)
// 	_, sub, err := c.PublishSubscribe(ctx, topic, info, ch)
// 	if err != nil {
// 		return err
// 	}

// 	ps, err := peerSet(ctx, sub, ch, n)
// 	if err != nil {
// 		return err
// 	}

// 	// connect edges
// 	g, ctx := errgroup.WithContext(ctx)
// 	for _, info := range t.Neighbors(h.ID(), ps) {
// 		g.Go(join(ctx, h, info))
// 	}

// 	return g.Wait()
// }

// // SelectDialPeers uses t to select bootstrap peers from the global peer set.
// // Use this with ww's boot.StaticAddrs
// func SelectDialPeers(ctx context.Context, c sync.Client, t Topology, n int) {

// }

// func peerSet(ctx context.Context, sub *sync.Subscription, ch <-chan peer.AddrInfo, n int) (PeerSet, error) {
// 	ps := make(PeerSet, 0, n)

// 	for i := 0; i < n; i++ {
// 		select {
// 		case info := <-ch:
// 			ps = append(ps, info)
// 		case err := <-sub.Done():
// 			if err != nil {
// 				return nil, errors.Wrap(err, "subscription closed")
// 			}
// 		case <-ctx.Done():
// 			return nil, ctx.Err()
// 		}
// 	}

// 	return ps, nil
// }

func join(ctx context.Context, h host.Host, info peer.AddrInfo) func() error {
	return func() error {
		return errors.Wrapf(h.Join(ctx, info), "join %s", info.ID)
	}
}

func min(x, y int) int {
	if x < y {
		return x
	}

	return y
}

type hostSetFactory struct {
	n   int
	sub *sync.Subscription
	ch  <-chan peer.AddrInfo
}

// Hosts that are present on the network.
func (a *hostSetFactory) Hosts(ctx context.Context) (PeerSet, error) {
	ps := make(PeerSet, 0, a.n)

	for i := 0; i < a.n; i++ {
		select {
		case info := <-a.ch:
			ps = append(ps, info)
		case err := <-a.sub.Done():
			if err != nil {
				return nil, errors.Wrap(err, "subscription closed")
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return ps, nil
}

func addrInfo(h host.Host) (info peer.AddrInfo, err error) {
	var as []multiaddr.Multiaddr
	if as, err = h.InterfaceListenAddrs(); err != nil {
		return
	}

	return peer.AddrInfo{
		ID:    h.ID(),
		Addrs: as,
	}, nil
}
