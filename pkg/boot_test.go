package lab

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wetware/ww/pkg/boot"
)

func TestLimit(t *testing.T) {
	for _, tC := range []struct {
		desc     string
		expected int
		opt      []boot.Option
		ps       PeerSet
	}{{
		desc:     "3",
		expected: 3,
		opt:      []boot.Option{boot.WithLimit(3)},
		ps:       make(PeerSet, 10),
	}, {
		desc:     "0",
		expected: 10,
		opt:      []boot.Option{boot.WithLimit(0)},
		ps:       make(PeerSet, 10),
	}} {
		var p boot.Param
		require.NoError(t, p.Apply(tC.opt),
			"test table contained invalid boot option")

		t.Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, tC.expected, limit(p, tC.ps))
		})
	}
}
