import copy
import random
from enum import IntEnum, auto
from typing import List

import click


class Policy(IntEnum):
    Random = auto()
    Pushpull = auto()
    Head = auto()

    @staticmethod
    def from_string(name: str):
        if name == "rand":
            return Policy.Random
        if name == "pushpull":
            return Policy.Pushpull
        if name == "head":
            return Policy.Head
        raise ValueError("Invalid policy name")


class Topology(IntEnum):
    RING = auto()

    def are_neighbors(self, i: int, j: int, nodes_amount: int):
        if self == Topology.RING:
            if i + 1 == j or j + 1 == i:
                return True
            if i == 0 and j == nodes_amount - 1:
                return True
            if j == 0 and i == nodes_amount - 1:
                return True
            return False


class Record:
    def __init__(self, index: int, hop: int):
        self.index = index
        self.hop = hop

    def __eq__(self, other):
        return isinstance(other, Record) and self.index == other.index

    def __str__(self):
        return f"Record<{self.index}:{self.hop}>"

    def __repr__(self):
        return str(self)


class Node:
    def __init__(self, index: int):
        self.index: int = index
        self.neighbors: List[Record] = []
        self._pull_buffer: List[Record] = []

    def __str__(self):
        return f"Node<{self.index}>"

    def __repr__(self):
        return str(self)

    @property
    def record(self):
        return Record(self.index, 0)

    def add_neighbor(self, neighbor: Record):
        self.neighbors.append(neighbor)

    def select_neighbors(self, selection: Policy, fanout: int):
        if selection is Policy.Random:
            return random.choices(self.neighbors, k=fanout)
        else:
            raise ValueError("Invalid selection policy")


class Cluster:
    def __init__(self, fanout: int, view_size: int, selection: Policy,
                 propagation: Policy, merge: Policy):
        self.fanout = fanout
        self.view_size = view_size
        self.selection = selection
        self.propagation = propagation
        self.merge = merge
        self.nodes = []

    def initialize_nodes(self, nodes_amount: int):
        for i in range(nodes_amount):
            node = Node(i)
            self.nodes.append(node)

    def initialize_topology(self, topology: Topology):
        for i, node in enumerate(self.nodes):
            for j, neighbor in enumerate(self.nodes):
                if i == j:
                    continue
                if topology.are_neighbors(i, j, len(self.nodes)):
                    record = neighbor.record
                    record.hop += 1
                    node.add_neighbor(record)

    def print_network(self):
        for node in self.nodes:
            print(f"{node} --> {node.neighbors}")

    def print_topology(self):
        for node in self.nodes:
            print(f"{node} --> {list(map(lambda r: self._to_node(r), node.neighbors))}")

    def simulate_tick(self, i: int):
        for node in self.nodes:
            for record in node.select_neighbors(self.selection, self.fanout):
                neighbor = self._to_node(record)
                if self.propagation is Policy.Pushpull:
                    self._push(node, neighbor)
                    self._push(neighbor, node)
                    self._pull(node, neighbor)
                    self._pull(neighbor, node)

    def _to_node(self, record: Record):
        return self.nodes[record.index]

    def _push(self, node: Node, neighbor: Node):
        neighbor._pull_buffer = copy.deepcopy(node.neighbors)
        neighbor._pull_buffer.append(node.record)

    def _pull(self, node: Node, neighbor: Node):
        for record in node._pull_buffer:
            record.hop += 1
        records = self._merge_records(node)
        if self.merge is Policy.Head:
            node.neighbors = sorted(records, key=lambda r: r.hop)
        node.neighbors = node.neighbors[:self.view_size]

    def _merge_records(self, node: Node) -> List[Record]:
        buffer: List[Record] = copy.deepcopy(node.neighbors)
        node._pull_buffer = list(filter(self._local_filter(node),
                                        node._pull_buffer))
        node._pull_buffer = list(filter(self._dedup_filter(buffer),
                                        node._pull_buffer))
        buffer = list(filter(self._dedup_filter(node._pull_buffer), buffer))
        buffer += node._pull_buffer
        return buffer

    def _dedup_filter(self, neighbors: List[Record]):
        def _filter(record: Record):
            for neighbor in neighbors:
                if neighbor == record and neighbor.hop <= record.hop:
                    return False
            return True

        return _filter

    def _local_filter(self, node: Node):
        def _filter(record: Record):
            return node.index != record.index

        return _filter


@click.command()
@click.argument("n", type=int)
@click.option("-t", "--ticks", type=int, default=30)
@click.option("-f", "--fanout", type=int, default=1)
@click.option("-v", "--view-size", type=int, default=32)
@click.option("-s", "--selection", type=str, default="rand")
@click.option("-p", "--propagation", type=str, default="pushpull")
@click.option("-m", "--merge", type=str, default="head")
def simulate(n: int, ticks: int, fanout: int, view_size: int,
             selection: str, propagation: str, merge: str):
    cluster = Cluster(fanout, view_size,
                      Policy.from_string(selection),
                      Policy.from_string(propagation),
                      Policy.from_string(merge))
    cluster.initialize_nodes(n)
    cluster.initialize_topology(Topology.RING)
    cluster.print_network()
    for i in range(ticks):
        cluster.simulate_tick(i)
        cluster.print_topology()

if __name__ == '__main__':
    simulate()
