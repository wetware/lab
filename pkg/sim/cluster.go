package sim

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
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

type Graph struct {
	Nodes []*Node `json:"nodes"`
	Links []*Link `json:"links"`
}

func (g Graph) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"type":  "graph",
		"nodes": len(g.Nodes),
		"links": len(g.Links),
	}
}

type Node struct {
	ID    string `json:"id"`
	Group int    `json:"group"`
}

func (n Node) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"type":  "node",
		"id":    n.ID,
		"group": n.Group,
	}
}

type Link struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int    `json:"value"`
}

func (l Link) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"type":   "link",
		"source": l.Source,
		"target": l.Target,
		"value":  l.Value,
	}
}

type Cluster struct {
	log log.Logger
	mu  sync.RWMutex
	env inproc.Env
	ps  map[peer.ID]member
	bus event.Bus
}

func NewCluster(log log.Logger) *Cluster {
	return &Cluster{
		log: log,
		env: inproc.NewEnv(),
		ps:  make(map[peer.ID]member),
		bus: eventbus.NewBus(),
	}
}

func (c *Cluster) Loggable() map[string]interface{} {
	return map[string]interface{}{
		// ...
	}
}

func (c *Cluster) Events(ctx context.Context) (event.Subscription, error) {
	return c.bus.Subscribe([]interface{}{
		new(Graph),
		new(Node),
		new(Link),
	})
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

	return g.Wait()
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

	sub, err := h.EventBus().Subscribe(new(net.EvtState))
	if err != nil {
		defer h.Close()
		defer o.Close()
		return "", err
	}

	if err := c.startEventLoop(sub.Out()); err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.ps[h.ID()] = member{Host: h, Net: o, sub: sub}
	return h.ID(), nil
}

func (c *Cluster) Kill(id peer.ID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.ps[id]; ok {
		defer delete(c.ps, id)
		return m.Close()
	}

	return errors.New("peer not found")
}

func (c *Cluster) startEventLoop(events <-chan interface{}) error {
	// e, err := c.bus.Emitter(new(net.EvtState))
	// if err != nil {
	// 	defer h.Close()
	// 	defer o.Close()
	// 	return "", err
	// }

	graph, err := c.bus.Emitter(new(Graph))
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile("internal/cmd/app/miserables.json")
	if err != nil {
		return err
	}

	var g Graph
	if err = json.Unmarshal(b, &g); err != nil {
		return err
	}

	if err = graph.Emit(g); err != nil {
		return err
	}

	go func() {
		defer graph.Close()

		for _ = range events {
			// _ = e.Emit(ev)
		}

	}()

	return nil
}

type member struct {
	Host host.Host
	Net  *net.Overlay

	sub io.Closer
}

func (m member) Close() error {
	return multierr.Combine(m.Net.Close(), m.Host.Close(), m.sub.Close())
}
