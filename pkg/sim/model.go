package sim

import (
	mapset "github.com/deckarep/golang-set"
	"github.com/libp2p/go-libp2p-core/event"
)

type model struct {
	nodes, links mapset.Set
	sub          event.Subscription
	export       event.Emitter
}

func newModel(sub event.Subscription, e event.Emitter) *model {
	m := &model{
		nodes:  mapset.NewThreadUnsafeSet(),
		links:  mapset.NewThreadUnsafeSet(),
		export: e,
		sub:    sub,
	}

	go m.loop()

	return m
}

func (m *model) Close() error { return m.sub.Close() }

func (m *model) loop() {
	defer m.export.Close()

	var (
		out StateChanged
	)

	for v := range m.sub.Out() {
		switch ev := v.(type) {
		case NodeChanged:
			switch ev.Event {
			case HostSpawned:
				if m.nodes.Add(ev.Peer) {
					out.Add = &Diff{Nodes: []*Node{{ID: ev.Peer}}}
				}

			case HostDied:
				if !m.nodes.Contains(ev.Peer) {
					continue
				}

				m.nodes.Remove(ev.Peer)
				out.Rm = &Diff{Nodes: []*Node{{ID: ev.Peer}}}

				m.nodes.Each(func(v interface{}) bool {
					if e := v.(Edge); e.Involves(ev.Peer) {
						m.links.Remove(v)
						out.Rm.Links = append(out.Rm.Links, e.ToLink())
					}
					return false
				})
			}

		case LinkChanged:
			switch ev.Event {
			case LinkCreated:
				if m.links.Add(ev.Peers) {
					out.Add = &Diff{Links: []*Link{ev.Peers.ToLink()}}
				}

			case LinkSevered:
				if !m.links.Contains(ev.Peers) {
					continue
				}

				m.links.Remove(ev.Peers)
				out.Rm = &Diff{Links: []*Link{ev.Peers.ToLink()}}

			}
		}

		// did something happen?
		if out != (StateChanged{}) {
			_ = m.export.Emit(out)
			out = StateChanged{} // clear
		}
	}
}
