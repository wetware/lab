package boot

import (
	"sort"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/wetware/casm/pkg/boot"
)

type Topology interface {
	GetNeighbors(boot boot.StaticAddrs) boot.StaticAddrs
}

type Ring struct{ ID peer.ID }

func (r Ring) GetNeighbors(as boot.StaticAddrs) boot.StaticAddrs {
	if len(as) < 2 {
		return as
	}

	sort.Sort(as)

	// Defensively range over the values instead of a 'while' loop,
	// in case the target ID is not contained in the slice.  This
	// should never happen, but you never know...
	for range as {
		if as[0].ID == r.ID {
			break
		}

		as = rotateLeft(as)
	}

	// use filter instead of indexing in case len(as) == 0
	return as.Filter(func(info peer.AddrInfo) bool {
		return info.ID != r.ID
	})[:2]
}

func rotateLeft(as boot.StaticAddrs) boot.StaticAddrs {
	return append(as[1:], as[0]) // always len(as) > 1
}
