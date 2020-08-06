package announce

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/testground/sdk-go/network"
	"github.com/testground/sdk-go/run"
	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"

	"github.com/wetware/ww/pkg/boot"
	"github.com/wetware/ww/pkg/client"
	"github.com/wetware/ww/pkg/host"
	wwrt "github.com/wetware/ww/pkg/runtime"
	"github.com/wetware/ww/pkg/runtime/service"

	lab "github.com/wetware/lab/pkg"
	"github.com/wetware/lab/pkg/graph"
)

var (
	stateInit = sync.State("init")
	stateDone = sync.State("done")
)

// RunTest tests cluster-wise peer announcement.  It verifies that hosts are mutually
// aware of each others' presence.
func RunTest(runenv *runtime.RunEnv, initc *run.InitContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	switch initc.SyncClient.MustSignalAndWait(ctx, stateInit, runenv.TestInstanceCount) {
	case 1:
		return subscribeClient(ctx, runenv, initc)
	default:
		return announceHost(ctx, runenv, initc)
	}
}

func subscribeClient(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	runenv.RecordMessage("I am the client")
	defer initc.SyncClient.MustSignalEntry(context.Background(), stateDone)

	c, err := dial(ctx, runenv, initc)
	if err != nil {
		return err
	}
	defer c.Close()

	if err = testAnnounce(ctx, runenv, c); err != nil {
		return errors.Wrap(err, "main test")
	}

	return nil
}

func announceHost(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h, err := mkhost(ctx, runenv, initc)
	if err != nil {
		return err
	}
	defer h.Close()

	if err = watchEvents(ctx, h, runenv); err != nil {
		return err
	}

	runenv.RecordMessage("%s is a host", h.ID())
	<-initc.SyncClient.MustBarrier(ctx, stateDone, 1).C // wait for client to terminate

	return nil
}

func dial(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) (c client.Client, err error) {
	b := lab.GraphJoiner{
		N:      runenv.TestInstanceCount - 1,
		Client: initc.SyncClient,
		T:      graph.Random(1),
	}

	return client.Dial(ctx,
		client.WithStrategy(b))
}

func mkhost(ctx context.Context, runenv *runtime.RunEnv, initc *run.InitContext) (h host.Host, err error) {
	var addr multiaddr.Multiaddr
	if addr, err = listenAddr(initc.NetClient); err != nil {
		return
	}

	if h, err = host.New(
		host.WithBootStrategy(boot.StaticAddrs{}),
		host.WithListenAddr(addr),
	); err != nil {
		return
	}

	if err = (lab.GraphBuilder{
		N:      runenv.TestInstanceCount - 1,
		Client: initc.SyncClient,
		TF:     graph.Line(),
	}).Build(ctx, h); err != nil {
		return
	}

	return h, nil
}

func testAnnounce(ctx context.Context, runenv *runtime.RunEnv, c client.Client) error {
	topic, err := c.Join("")
	if err != nil {
		return errors.Wrap(err, "join topic")
	}
	defer topic.Close()

	sub, err := topic.Subscribe(ctx)
	if err != nil {
		return errors.Wrap(err, "subscribe topic")
	}

	ps := make(map[peer.ID]struct{})
	for msg := range sub.C {
		if _, ok := ps[msg.GetFrom()]; ok {
			continue
		}

		runenv.RecordMessage("got entry for %s", msg.GetFrom())

		// loop until at least one message from all peers was found.
		if ps[msg.GetFrom()] = struct{}{}; len(ps) == runenv.TestInstanceCount-1 {
			break
		}
	}

	return nil
}

func listenAddr(nc *network.Client) (multiaddr.Multiaddr, error) {
	ip, err := nc.GetDataNetworkIP()
	if err != nil {
		return nil, err
	}

	var str string
	if isIP4(ip) {
		str = fmt.Sprintf("/ip4/%s/tcp/0", ip)
	} else {
		str = fmt.Sprintf("/ip6/%s/tcp/0", ip)
	}

	return multiaddr.NewMultiaddr(str)
}

func watchEvents(ctx context.Context, h host.Host, runenv *runtime.RunEnv) error {
	sub, err := h.EventBus().Subscribe([]interface{}{
		new(wwrt.Exception),
		new(service.EvtPeerDiscovered),
		new(service.EvtNeighborhoodChanged),
	})
	if err == nil {
		go func() {
			for v := range sub.Out() {
				switch ev := v.(type) {
				case wwrt.Exception:
					runenv.RecordFailure(ev)
				case service.EvtPeerDiscovered:
					runenv.SLogger().Infof("discovered %s", peer.AddrInfo(ev))
				case service.EvtNeighborhoodChanged:
					runenv.SLogger().Infof("from %d to %d", ev.From, ev.To)
				}
			}
		}()
	}

	return err
}

func isIP4(ip net.IP) bool {
	return !(ip.To4() == nil)
}
