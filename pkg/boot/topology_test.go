package boot_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	cboot "github.com/wetware/casm/pkg/boot"
	"github.com/wetware/lab/pkg/boot"
)

func TestRing(t *testing.T) {
	t.Parallel()

	as := newStaticAddrs(8)
	topo := boot.Ring{ID: as[0].ID}
	top := topo.GetNeighbors(as)
	require.Len(t, top, 7)
	require.NotContains(t, top, topo.ID,
		"should NOT have local host in results")

	require.Equal(t, as[0:1], topo.GetNeighbors(as[0:1]),
		"should return length-1 array unchanged")

	require.Equal(t,
		cboot.StaticAddrs(nil),
		cboot.StaticAddrs(nil),
		"should return length-0 array unchanged")
}
