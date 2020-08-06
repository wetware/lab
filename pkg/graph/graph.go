// Package graph contains commonly-used graph topology definitions & utilities
package graph

import (
	"math/rand"
	"sort"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	lab "github.com/wetware/lab/pkg"
)

// TopologyFunc satisfies Topology
type TopologyFunc func(lab.PeerSet) lab.PeerSet

// Neighbors for the local peer.
func (f TopologyFunc) Neighbors(ps lab.PeerSet) lab.PeerSet {
	return f(ps)
}

// TopologyFactoryFunc satisfies TopologyFactory
type TopologyFactoryFunc func(peer.ID) lab.Topology

// NewTopology for the peer
func (f TopologyFactoryFunc) NewTopology(peer peer.ID) lab.Topology {
	return f(peer)
}

// Random topology establishes n random connections to the local peer.
func Random(n int) TopologyFunc {
	return func(ps lab.PeerSet) lab.PeerSet {
		rand.Shuffle(len(ps), func(i, j int) {
			ps[i], ps[j] = ps[j], ps[i]
		})

		return ps[:n]
	}
}

// Line topology.
func Line() TopologyFactoryFunc {
	return func(id peer.ID) lab.Topology {
		return TopologyFunc(func(ps lab.PeerSet) lab.PeerSet {
			ps = Sorted(ps)

			m := idxmap(ps)
			idx, ok := m[id]
			if !ok {
				panic(errors.Errorf("%s not in peer set", id))
			}

			switch idx {
			case len(ps) - 1:
				return lab.PeerSet{}
			default:
				return lab.PeerSet{ps[idx+1]}
			}
		})
	}
}

// Ring topology.
func Ring() TopologyFactoryFunc {
	return func(id peer.ID) lab.Topology {
		return TopologyFunc(func(ps lab.PeerSet) lab.PeerSet {
			ns := make(lab.PeerSet, 0, 2)
			ps = Sorted(ps)

			m := idxmap(ps)
			idx, ok := m[id]
			if !ok {
				panic(errors.Errorf("%s not in peer set", id))
			}

			// left peer
			switch idx {
			case 0:
				ns = append(ns, ps[len(ps)-1])
			default:
				ns = append(ns, ps[idx-1])
			}

			// right peer
			switch idx {
			case len(ps) - 1:
				ns = append(ns, ps[0])
			default:
				ns = append(ns, ps[idx+1])
			}

			return ns
		})
	}
}

// Sorted returns a copy of ps that is sorted by peer.ID
func Sorted(ps lab.PeerSet) lab.PeerSet {
	sorted := copyPeers(ps)
	sort.Sort(sorted)
	return sorted
}

func idxmap(ps lab.PeerSet) map[peer.ID]int {
	m := make(map[peer.ID]int, len(ps))
	for i, info := range ps {
		m[info.ID] = i
	}
	return m
}

func copyPeers(ps lab.PeerSet) lab.PeerSet {
	copied := make(lab.PeerSet, len(ps))
	copy(copied, ps)
	return copied
}

func filter(id peer.ID, ps lab.PeerSet) lab.PeerSet {
	ps = copyPeers(ps)

	out := ps[:0]
	for _, info := range ps {
		if info.ID != id {
			out = append(out, info)
		}
	}

	return out
}
