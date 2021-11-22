import numpy as np
import networkx as nx
from collections import Counter
import copy
import os.path
import random
import shlex
import string
import subprocess
from datetime import datetime
from enum import IntEnum, auto
from typing import List, Dict

import click
from influxdb import InfluxDBClient


@click.group()
def cli1():
    pass


@click.group()
def cli2():
    pass


cli = click.CommandCollection(sources=[cli1, cli2])

nodes: List["Node"] = []


class Policy(IntEnum):
    Rand = auto()
    Pushpull = auto()
    Head = auto()

    @staticmethod
    def from_string(name: str):
        if name == "rand":
            return Policy.Rand
        if name == "pushpull":
            return Policy.Pushpull
        if name == "head":
            return Policy.Head
        raise ValueError("Invalid policy name")


class Topology(IntEnum):
    RING = auto()
    RAND = auto()

    @staticmethod
    def from_string(name: str):
        if name == "rand":
            return Topology.RAND
        if name == "ring":
            return Topology.RING
        raise ValueError("Invalid topology name")

    def are_neighbors(self, i: int, j: int, nodes_amount: int):
        if self == Topology.RING:
            if i + 1 == j or j + 1 == i:
                return True
            if i == 0 and j == nodes_amount - 1:
                return True
            if j == 0 and i == nodes_amount - 1:
                return True
        if self is Topology.RAND:
            r = random.Random()
            r.seed(1234)
            index = [k for k in range(nodes_amount)]
            r.shuffle(index)
            i, j = index[i], index[j]
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
        return isinstance(other, Record) and self.index == other.index and self.hop == other.hop

    def __str__(self):
        return f"Record<{self.index}:{self.hop}>"

    def __repr__(self):
        return str(self)

    def copy(self) -> "Record":
        return Record(self.index, self.hop)


class Node:
    def __init__(self, index: int):
        self.index: int = index
        self.neighbors: List[Record] = []
        self._pull_buffer: List[Record] = []
        self._cluster: Cluster = None

    def __str__(self):
        return f"Node<{self.index}>"

    def __repr__(self):
        return str(self)

    @property
    def record(self):
        return Record(self.index, 0)

    @staticmethod
    def from_record(record: Record) -> "Node":
        return nodes[record.index]

    def set_neighbors(self, neighbors: List[Record]):
        self.neighbors = neighbors

    def add_neighbor(self, neighbor: Record):
        self.neighbors.append(neighbor)

    def del_neighbor(self, neighbor: Record):
        assert neighbor in self.neighbors
        self.neighbors.remove(neighbor)
        assert neighbor not in self.neighbors

    def set_cluster(self, cluster: "Cluster"):
        self._cluster = cluster

    def select_neighbors(self, selection: Policy, fanout: int):
        if selection is Policy.Rand:
            return random.sample(self.neighbors, k=fanout)
        else:
            raise ValueError("Invalid selection policy")


class Cluster:
    next_id = 0

    def __init__(self, fanout: int, c: int, selection: Policy,
                 propagation: Policy, merge: Policy, H: int, S: int, R: int, X: int, E: bool):
        self.fanout = fanout
        self.c = c
        self.selection = selection
        self.propagation = propagation
        self.merge = merge
        self.H = H
        self.S = S
        self.R = R
        self.X = X
        self.E = E
        self.graph = nx.DiGraph()
        self.nodes: Dict[int, Node] = {}
        self.tick = 0
        self.id = Cluster.next_id
        Cluster.next_id += 1

    def initialize_nodes(self, nodes: List[Node]):
        for node in nodes:
            self.nodes[node.index] = node
            node.set_cluster(self)
            if self.graph.has_node(node.index):
                self.graph.nodes[node.index]["cluster"] = self.id
            else:
                self.graph.add_node(node.index, cluster=self.id)

    def initialize_topology(self, topology: Topology):
        for i, node in self.nodes.items():
            for j, neighbor in self.nodes.items():
                if i == j:
                    continue
                if topology.are_neighbors(i, j, len(self.nodes)):
                    record = neighbor.record
                    record.hop += 1
                    node.add_neighbor(record)
                    self.graph.add_edge(node.index, neighbor.index)

    def print_network(self):
        for node in self.nodes.values():
            print(f"{node} --> {node.neighbors}")

    def print_topology(self):
        for node in self.nodes.values():
            print(f"{node} --> {list(map(lambda r: self._to_node(r), node.neighbors))}")

    def simulate_tick(self, i: int):
        for node in self.nodes.values():
            for record in node.select_neighbors(self.selection, self.fanout):
                neighbor = self._to_node(record)
                if not neighbor:
                    if self.E:
                        node.del_neighbor(record)
                        self.graph.remove_edge(node.index, record.index)
                    continue
                if self.propagation is Policy.Pushpull:
                    self._push(node, neighbor)
                    self._push(neighbor, node)
                    self._pull(node, neighbor)
                    self._pull(neighbor, node)

    def partition(self, nodes: List[Node]) -> "Cluster":
        partition = Cluster(self.fanout, self.c, self.selection,
                            self.propagation, self.merge, self.H, self.S, self.R,
                            self.X, self.E)
        partition.graph = self.graph
        partition.initialize_nodes(nodes)
        for node in nodes:
            self.nodes.pop(node.index)
        return partition

    def _to_node(self, record: Record):
        return self.nodes.get(record.index)

    def _push(self, node: Node, neighbor: Node):
        c = min(self.c // 2, len(node.neighbors))
        neighbor._pull_buffer = [n.copy() for n in node.neighbors[:c]]
        neighbor._pull_buffer.append(node.record)

    def _pull(self, node: Node, neighbor: Node):
        records = self._merge_records(node)

        R = min(self.R, len(records), self.c)
        c = self.c - R
        S = min(self.S, max(len(records) - c, 0))
        records = records[S:]

        H = min(self.H, max(len(records) - c, 0))
        records = sorted(records, key=lambda r: r.hop, reverse=True)
        oldest, records = records[:R], records[R + H:]
        X = min(self.X, max(len(records) - c, 0))
        if X:
            np.random.shuffle(records)
        records = records[X:] + oldest
        np.random.shuffle(records)
        records = records[:self.c]

        node._pull_buffer = []

        s_old, s_new = set(map(lambda n: n.index, node.neighbors)), set(map(lambda n: n.index, records))
        for i in s_old - s_new:
            self.graph.remove_edge(node.index, i)
        for i in s_new - s_old:
            self.graph.add_edge(node.index, i)

        node.set_neighbors(records)
        for neighbor in node.neighbors:
            neighbor.hop += 1

    def _merge_records(self, node: Node) -> List[Record]:
        buffer: List[Record] = []
        pull_set = dict((r.index, r) for r in node._pull_buffer)
        for r1 in node.neighbors:
            r2 = pull_set.get(r1.index, None)
            if not r2 or r1.hop <= r2.hop:
                buffer.append(r1)

        buffer_set = dict((r.index, r) for r in buffer)
        for r1 in node._pull_buffer:
            r2 = buffer_set.get(r1.index, None)
            if r1.index != node.index and not r2:
                buffer.append(r1)

        return buffer

    def _dedup_filter(self, neighbors: List[Record]):
        def _filter(record: Record):
            for neighbor in neighbors:
                if neighbor.index == record.index and neighbor.hop <= record.hop:
                    return False
            return True

        return _filter

    def _local_filter(self, node: Node):
        def _filter(record: Record):
            return node.index != record.index

        return _filter

    def __str__(self):
        return f"Cluster<{self.id}:{len(self.nodes)}>"

    def __repr__(self):
        return str(self)


class PartitionType(IntEnum):
    Rand = auto()
    Lineal = auto()

    @staticmethod
    def from_string(name: str):
        if name == "rand":
            return PartitionType.Rand
        if name == "lineal":
            return PartitionType.Lineal
        raise ValueError("Invalid partition type name")

    def partition_nodes(self, nodes: List[Node], partition_size: int) -> List[Node]:
        if self is PartitionType.Rand:
            return random.sample(nodes, k=min(len(nodes), partition_size))
        elif self is PartitionType.Lineal:
            return nodes[:min(len(nodes), partition_size)]
        raise ValueError("Invalid partition type")


def write_to_influx(cluster: Cluster, run_id: str, tick: int):
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")
    json_body = []
    for node in cluster.nodes.values():
        point = {
            "measurement": "diagnostics.casm-pex-convergence.view.point",
            "tags": {
                "node": str(node.index),
                "records": "-".join(map(lambda r: str(r.index), node.neighbors)),
                "tick": tick,
                "run": run_id,
                "cluster": node._cluster.id
            },
            "time": datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ'),
            "fields": {
                "value": 0.0
            }
        }
        json_body.append(point)
    client.write_points(json_body)


def write_info_to_file(run_id: str, params: Dict[str, int], folder: str):
    folder = os.path.join(folder, run_id) if folder else run_id
    output_file = os.path.join(folder, f"info.sim")
    if not os.path.isdir(folder):
        os.mkdir(folder)
    with open(output_file, "w") as file:
        for param, value in params.items():
            file.write(f"{param}={value}\n")


def write_to_file(clusters: List[Cluster], run_id: str, tick: int, folder: str):
    folder = os.path.join(folder, run_id) if folder else run_id
    output_file = os.path.join(folder, f"{run_id}.{tick}.partition.sim")
    if not os.path.isdir(folder):
        os.mkdir(folder)

    nx.write_gpickle(clusters[0].graph, output_file)


def init_metrics():
    pass  # TODO


@click.command()
@click.option("-t", "--ticks", type=int, default=50)
@click.option("-r", "--repetitions", type=int, default=1)
@click.option("-s", "--step", type=int, default=1)
@click.option("-min", "--min-nodes", type=int, default=3)
@click.option("-max", "--max-nodes", type=int, default=3)
@click.option("-tp", "--topology", type=str, default="ring")
@click.option("-f", "--fanout", type=int, default=1)
@click.option("-c", type=int, default=32)
@click.option("-sp", "--selection", type=str, default="rand")
@click.option("-pp", "--propagation", type=str, default="pushpull")
@click.option("-mp", "--merge", type=str, default="head")
@click.option("-H", type=int, default=0)
@click.option("-S", type=int, default=0)
@click.option("-R", type=int, default=0)
@click.option("-X", type=int, default=0)
@click.option("-E", is_flag=True)
@click.option("-p", "--partition", multiple=True, type=str)
@click.option("-ptp", "--partition-type", type=str, default="rand")
@click.option("--influx", is_flag=True)
@click.option('-f', '--folder', help="Output folder to store the file to.",
              type=str)
@click.option('--plot', help="Plot simulation convergence graph.",
              is_flag=True)
def simulate(ticks: int, repetitions: int, step: int, fanout: int, min_nodes: int,
             max_nodes: int, topology: str, c: int, selection: str, propagation: str,
             merge: str, h: int, s: int, r: int, x: int, e: bool, partition: List[str], partition_type: str, influx: bool,
             folder: str, plot: bool):
    simulation_id = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    output_file = os.path.join(folder,
                               f"{simulation_id}.partition.pex.sim") if folder else f"{simulation_id}.partition.pex.sim"
    max_nodes = max(min_nodes, max_nodes)
    with open(output_file, "a") as file:
        file.write(f"{min_nodes} {max_nodes} {repetitions} {step}\n")
    init_metrics()
    partitions = [(int(p.split(":")[0]), int(p.split(":")[1]) )for p in partition]
    partition_type = PartitionType.from_string(partition_type)

    for n in range(min_nodes, max_nodes + 1, step):
        for i in range(repetitions):
            global nodes
            run_id = "".join(random.choices(string.
                                            ascii_lowercase + string.digits, k=16))
            write_info_to_file(run_id, {"H": h, "S": s, "R": r, "X": x, "c": c}, folder)

            nodes = [Node(node_id) for node_id in range(n)]
            clusters = []
            c0 = Cluster(fanout, c,
                         Policy.from_string(selection),
                         Policy.from_string(propagation),
                         Policy.from_string(merge), h, s, r, x, e)
            clusters.append(c0)
            c0.initialize_nodes(nodes)
            c0.initialize_topology(Topology.from_string(topology))

            print(f"{n} - Run {run_id} ({i + 1}/{repetitions}) started")
            for tick in range(1, ticks + 1):
                print(f"N={n}({i + 1}/{repetitions}) - Tick {tick}/{ticks}...")
                for partition_tick, partition_size in partitions:
                    if partition_tick and tick == partition_tick:
                        partition = partition_type.partition_nodes(list(c0.nodes.values()), partition_size)
                        clusters.append(c0.partition(partition))
                        print(f"Partitioned: {clusters}")
                for c in clusters:
                    c.simulate_tick(tick)
                    if influx:
                        write_to_influx(c, run_id, tick)
                if not influx:
                    write_to_file(clusters, run_id, tick, folder)
            print(f"{n} - Run {run_id} ({i + 1}/{repetitions}) finished - {clusters}")

            with open(output_file, "a") as file:
                file.write(f"{os.path.join(folder, run_id)}\n")
    print(f"Results stored at {output_file}")

    if plot:
        subprocess.run(shlex.split(f"python3 convergence.py plot {output_file}"))


if __name__ == '__main__':
    simulate()
