package boot

import (
	"sort"

	"github.com/libp2p/go-libp2p-core/peer"
	pexboot "github.com/wetware/casm/pkg/boot"
)

type Topology interface{
	Name() string
	GetNeighbors(p peer.ID, boot pexboot.StaticAddrs) pexboot.StaticAddrs
}

type Ring struct{}

func (r *Ring) Name() string{
	return "ring"
}

func (r *Ring) GetNeighbors(p peer.ID, boot pexboot.StaticAddrs) pexboot.StaticAddrs{
	sort.Sort(boot)
	for boot[0].ID != p {
		boot = rotateLeft(boot)
	}
	
	return pexboot.StaticAddrs{boot[1], boot[len(boot)-1]}
}

func rotateLeft(boot pexboot.StaticAddrs) pexboot.StaticAddrs{
	return append(boot[1:], boot[0])
}