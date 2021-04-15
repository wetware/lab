package sim

import (
	"context"
	"errors"
	"sync"

	"github.com/libp2p/go-eventbus"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	inproc "github.com/lthibault/go-libp2p-inproc-transport"
	"github.com/lthibault/log"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"

	"github.com/wetware/casm/pkg/net"
)

type Cluster struct {
	log   log.Logger
	mu    sync.RWMutex
	env   inproc.Env
	ps    map[peer.ID]*member
	bus   event.Bus
	event event.Emitter // MembershipChanged
	model *model
}

func NewCluster(log log.Logger) (*Cluster, error) {
	bus := eventbus.NewBus()

	graphSub, err := bus.Subscribe([]interface{}{
		new(NodeChanged),
		new(LinkChanged),
	})
	if err != nil {
		return nil, err
	}

	export, err := bus.Emitter(new(StateChanged))
	if err != nil {
		return nil, err
	}

	membership, err := bus.Emitter(new(NodeChanged))
	if err != nil {
		return nil, err
	}

	c := &Cluster{
		log:   log,
		env:   inproc.NewEnv(),
		ps:    make(map[peer.ID]*member),
		bus:   bus,
		event: membership,
		model: newModel(graphSub, export),
	}

	return c, nil
}

func (c *Cluster) Loggable() map[string]interface{} {
	return map[string]interface{}{
		// ...
	}
}

func (c *Cluster) Events(ctx context.Context) (event.Subscription, error) {
	return c.bus.Subscribe(new(StateChanged))
}

func (c *Cluster) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var g errgroup.Group
	for id := range c.ps {
		g.Go(func(id peer.ID) func() error {
			return func() error { return c.Kill(id) }
		}(id))
	}
	err := g.Wait()

	return multierr.Combine(err,
		c.event.Close(),
		c.model.Close())
}

func (c *Cluster) Spawn(ctx context.Context) (peer.ID, error) {
	h, err := libp2p.New(ctx,
		libp2p.NoTransports,
		libp2p.Transport(inproc.New(inproc.WithEnv(c.env))))
	if err != nil {
		return "", err
	}

	o, err := net.New(h)
	if err != nil {
		defer h.Close()
		return "", err
	}

	// subscribe to host events
	sub, err := h.EventBus().Subscribe(new(net.EvtState))
	if err != nil {
		defer h.Close()
		defer o.Close()
		return "", err
	}

	// bridge host events to the cluster bus
	e, err := c.bus.Emitter(new(LinkChanged))
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.ps[h.ID()] = newMember(h, o, sub, e)
	defer c.log.WithField("peer", h.ID()).Info("spawned host")

	return h.ID(), c.event.Emit(NodeChanged{
		Event: HostSpawned,
		Peer:  h.ID(),
	})
}

func (c *Cluster) Kill(id peer.ID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	m, ok := c.ps[id]
	if !ok {
		return errors.New("peer not found")
	}

	defer delete(c.ps, id)
	defer c.event.Emit(NodeChanged{
		Event: HostDied,
		Peer:  id,
	})

	return m.Close()

}

type member struct {
	Host host.Host
	Net  *net.Overlay

	sub event.Subscription
}

func newMember(h host.Host, o *net.Overlay, sub event.Subscription, e event.Emitter) *member {
	m := &member{Host: h, Net: o, sub: sub}
	go m.loop(e)
	return m
}

func (m member) Close() error {
	return multierr.Combine(m.Net.Close(), m.Host.Close(), m.sub.Close())
}

func (m member) loop(e event.Emitter) {
	defer e.Close()

	var (
		out LinkChanged
		ev  net.EvtState
	)

	for v := range m.sub.Out() {
		switch ev = v.(net.EvtState); ev.Event {
		case net.EventJoined:
		case net.EventLeft:
		}

		out.Peers = link(m.Host.ID(), ev.Peer)
		_ = e.Emit(out)
	}
}
