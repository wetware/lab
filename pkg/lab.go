package lab

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/testground/sdk-go/sync"
)

var (
	topic = sync.NewTopic("peerset", new(peer.AddrInfo))
)

// TopologyFactory builds the peer's local topology.
type TopologyFactory interface {
	NewTopology(peer.ID) Topology
}

// Topology provides the local peer with a list of immediate neighbors to
// which it should attempt a connection.  It is used by RedisStrategy in order to
// instantiate arbitrary graph topologies for testing.
type Topology interface {
	Neighbors(peers PeerSet) (neighbors PeerSet)
}

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
