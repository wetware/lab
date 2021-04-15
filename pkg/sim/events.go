package sim

import (
	"sort"

	"github.com/libp2p/go-libp2p-core/peer"
)

type StateChanged struct {
	Add *Diff `json:"add"`
	Rm  *Diff `json:"rm"`
}

type Diff struct {
	Nodes []*Node `json:"nodes"`
	Links []*Link `json:"links"`
}

type Node struct {
	ID peer.ID `json:"id"`
}

type Link struct {
	Source peer.ID `json:"source"`
	Target peer.ID `json:"target"`
}

type NodeEvent uint8

const (
	HostSpawned NodeEvent = iota
	HostDied
)

type NodeChanged struct {
	Event NodeEvent
	Peer  peer.ID
}

type LinkEvent uint8

const (
	LinkCreated LinkEvent = iota
	LinkSevered
)

type LinkChanged struct {
	Event LinkEvent
	Peers Edge
}

type Edge peer.IDSlice

func (e Edge) Involves(id peer.ID) bool {
	for _, x := range e {
		if x == id {
			return true
		}
	}
	return false
}

func (e Edge) ToLink() *Link {
	return &Link{
		Source: e[0],
		Target: e[1],
	}
}

func link(x, y peer.ID) Edge {
	ids := peer.IDSlice{x, y}
	sort.Sort(ids)
	return Edge(ids)
}
