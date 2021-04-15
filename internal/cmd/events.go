package cmd

import (
	"github.com/google/uuid"
	"github.com/wetware/lab/pkg/sim"
)

type SimulationChanged struct {
	Graph *Graph            `json:"graph,omitempty"`
	Step  *sim.StateChanged `json:"step,omitempty"`
}

type Graph struct {
	Cluster uuid.UUID   `json:"id"`
	Nodes   []*sim.Node `json:"nodes"`
	Links   []*sim.Link `json:"links"`
}
