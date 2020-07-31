// Package topology contains commonly-used graph topology definitions & utilities
package topology

import (
	"math/rand"
	"sort"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	lab "github.com/wetware/lab/pkg"
)

// ConstructorFunc satisfies TopologyConstructor
type ConstructorFunc func(peer.ID, lab.PeerSet) lab.PeerSet

// Neighbors for the local peer.
func (f ConstructorFunc) Neighbors(id peer.ID, ps lab.PeerSet) lab.PeerSet {
	return f(id, ps)
}

// Random topology establishes n random connections to the local peer.
func Random(n int) ConstructorFunc {
	return func(id peer.ID, ps lab.PeerSet) lab.PeerSet {
		ps = filter(id, ps)

		rand.Shuffle(len(ps), func(i, j int) {
			ps[i], ps[j] = ps[j], ps[i]
		})

		return ps[:n]
	}
}

// Line topology.
func Line() ConstructorFunc {
	return func(id peer.ID, ps lab.PeerSet) lab.PeerSet {
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
	}
}

// Ring topology.
func Ring() ConstructorFunc {
	return func(id peer.ID, ps lab.PeerSet) lab.PeerSet {
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
