package boot_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"
	"github.com/testground/sdk-go/sync"
	cboot "github.com/wetware/casm/pkg/boot"
	"github.com/wetware/lab/pkg/boot"
)

func TestRedisDiscovery(t *testing.T) {
	t.Parallel()

	const ns = "test"

	var (
		as    = newStaticAddrs(8)
		c     = sync.NewInmemClient()
		topic = sync.NewTopic(ns, new(peer.AddrInfo))
	)

	for _, info := range as {
		_ = c.MustPublish(context.Background(), topic, &info)
	}

	d := &boot.RedisDiscovery{
		C:     c,
		Local: newAddrInfo(),
	}

	ch, err := d.FindPeers(context.Background(), ns)
	require.NoError(t, err)
	require.Empty(t, ch)

}

func newStaticAddrs(n int) cboot.StaticAddrs {
	as := make(cboot.StaticAddrs, 0, n)
	for i := 0; i < n; i++ {
		as = append(as, *newAddrInfo())
	}
	return as
}

func newAddrInfo() *peer.AddrInfo {
	return &peer.AddrInfo{
		ID: newPeerID(),
	}
}

func newPeerID() peer.ID {
	// use non-cryptographic source; it's just a test.
	sk, _, err := crypto.GenerateECDSAKeyPair(rand.Reader)
	if err != nil {
		panic(err)
	}

	id, err := peer.IDFromPrivateKey(sk)
	if err != nil {
		panic(err)
	}

	return id
}
