package lab

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/wetware/ww/pkg/boot"
)

var (
	topic          = sync.NewTopic("peers", new(peer.AddrInfo))
	statePublished = sync.State("published")
)

// PeerSet is a set of peer.AddrInfos
type PeerSet []peer.AddrInfo

func (ps PeerSet) Len() int {
	return len(ps)
}

func (ps PeerSet) Less(i, j int) bool {
	return string(ps[i].ID) < string(ps[j].ID)
}

func (ps PeerSet) Swap(i, j int) {
	ps[i], ps[j] = ps[j], ps[i]
}

// TopologyConstructor provides the local peer with a list of immediate neighbors to
// which it should attempt a connection.  It is used by Bootstrapper in order to
// instantiate arbitrary graph topologies for testing.
type TopologyConstructor interface {
	Neighbors(local peer.ID, peers PeerSet) (neighbors PeerSet)
}

// Bootstrapper implements boot.Strategy & boot.Beacon.
type Bootstrapper struct {
	r    *runtime.RunEnv
	c    sync.Client
	t    TopologyConstructor
	info chan *peer.AddrInfo
}

// Boot a cluster using github.com/testground/sdk-go/sync.
// It does not close the underlying sync.Client.
func Boot(r *runtime.RunEnv, c sync.Client, t TopologyConstructor) Bootstrapper {
	return Bootstrapper{
		r:    r,
		c:    c,
		t:    t,
		info: make(chan *peer.AddrInfo, 1),
	}
}

// Loggable representation of the bootstrapper
func (b Bootstrapper) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"service": "lab.boot",
	}
}

// DiscoverPeers over Testground sync service.
func (b Bootstrapper) DiscoverPeers(ctx context.Context, opt ...boot.Option) (<-chan peer.AddrInfo, error) {
	var p boot.Param
	if err := p.Apply(opt); err != nil {
		return nil, err
	}

	// Wait for all peers to have published their presence
	b.r.RecordMessage("synchronizing ...")
	if err := b.waitPeers(ctx); err != nil {
		return nil, err
	}
	b.r.RecordMessage("ready")

	// Wait for all peers to signal their presence on the network
	ps, err := b.peers(ctx)
	if err != nil {
		return nil, err
	}

	select {
	case info := <-b.info:
		// Select the subset of peers to whom we should connect.
		// Note that this applies boot parameters such as `Limit`.
		return b.neighbors(ps, p, info.ID), nil
	case <-ctx.Done():
		// DiscoverPeers is called concurrently with `Signal`; If the latter call fails,
		// we will never receive the AddrInfo, so we must have a timeout mechanism.
		return nil, ctx.Err()
	}
}

// Signal the host's presence on the network.
func (b Bootstrapper) Signal(ctx context.Context, h host.Host) error {
	info, err := addrInfoPtr(h)
	if err != nil {
		return err
	}

	b.info <- info // buffered

	// publish our presence on the network
	if _, err = b.c.Publish(ctx, topic, info); err != nil {
		return err
	}

	// send synchronization signal, which will be picked up by other hosts AND clients
	_, err = b.c.SignalEntry(ctx, statePublished)
	return err
}

// Stop the active service advertisement the beacon
func (b Bootstrapper) Stop(context.Context) error {
	return nil
}

func (b Bootstrapper) peers(ctx context.Context) (PeerSet, error) {
	ch := make(chan peer.AddrInfo, 1)

	sub, err := b.c.Subscribe(ctx, topic, ch)
	if err != nil {
		return nil, err
	}

	var ps PeerSet
	for i := 0; i < b.r.TestInstanceCount-1; i++ {
		select {
		case info := <-ch:
			ps = append(ps, info)
		case err = <-sub.Done():
			return nil, err
		}
	}

	return ps, nil
}

func (b Bootstrapper) neighbors(ps PeerSet, p boot.Param, id peer.ID) <-chan peer.AddrInfo {
	ps = b.t.Neighbors(id, ps)
	ch := make(chan peer.AddrInfo, limit(p, ps))
	defer close(ch)

	for _, info := range ps {
		select {
		case ch <- info:
		default:
			// channel is full; drop it on the floor
		}
	}

	b.r.RecordMessage("neighbors: %s", ps)
	return ch
}

func (b Bootstrapper) waitPeers(ctx context.Context) error {
	bar, err := b.c.Barrier(ctx, statePublished, b.r.TestInstanceCount-1)
	if err != nil {
		return err
	}

	return <-bar.C // will close if ctx expires
}

func addrInfoPtr(h host.Host) (*peer.AddrInfo, error) {
	as, err := h.Network().InterfaceListenAddresses()
	if err != nil {
		return nil, err
	}

	// pointer is needed because b.SyncClient.Publish requires a pointer type.
	return &peer.AddrInfo{
		ID:    h.ID(),
		Addrs: as,
	}, nil
}

func min(x, y int) int {
	if x < y {
		return x
	}

	return y
}

func limit(p boot.Param, ps PeerSet) int {
	if p.Limit == 0 {
		return len(ps)
	}

	return min(len(ps), p.Limit)
}
