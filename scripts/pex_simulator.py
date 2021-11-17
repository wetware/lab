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
        return isinstance(other, Record) and self.index == other.index and self.hop == other.hop

    def __str__(self):
        return f"Record<{self.index}:{self.hop}>"

    def __repr__(self):
        return str(self)


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
        self.neighbors.remove(neighbor)

    def set_cluster(self, cluster: "Cluster"):
        self._cluster = cluster

    def select_neighbors(self, selection: Policy, fanout: int):
        if selection is Policy.Rand:
            return random.sample(self.neighbors, k=fanout)
        else:
            raise ValueError("Invalid selection policy")


class Cluster:
    next_id = 0

    def __init__(self, fanout: int, view_size: int, selection: Policy,
                 propagation: Policy, merge: Policy):
        self.fanout = fanout
        self.view_size = view_size
        self.selection = selection
        self.propagation = propagation
        self.merge = merge
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
            exchange_amount = 0
            while exchange_amount < self.fanout and node.neighbors:
                for record in node.select_neighbors(self.selection, self.fanout):
                    neighbor = self._to_node(record)
                    if not neighbor:
                        node.del_neighbor(record)
                        self.graph.remove_edge(node.index, record.index)
                        continue
                    if self.propagation is Policy.Pushpull:
                        self._push(node, neighbor)
                        self._push(neighbor, node)
                        self._pull(node, neighbor)
                        self._pull(neighbor, node)
                    exchange_amount += 1

    def partition(self, nodes: List[Node]) -> "Cluster":
        partition = Cluster(self.fanout, self.view_size, self.selection, self.propagation, self.merge)
        partition.graph = self.graph
        partition.initialize_nodes(nodes)
        for node in nodes:
            self.nodes.pop(node.index)
        return partition

    def _to_node(self, record: Record):
        return self.nodes.get(record.index)

    def _push(self, node: Node, neighbor: Node):
        neighbor._pull_buffer = copy.deepcopy(node.neighbors)
        neighbor._pull_buffer.append(node.record)

    def _pull(self, node: Node, neighbor: Node):
        for record in node._pull_buffer:
            record.hop += 1
        records = self._merge_records(node)
        if self.merge is Policy.Head:
            records = sorted(records, key=lambda r: r.hop)
        if self.merge is Policy.Rand:
            random.shuffle(records)
        node._pull_buffer = []
        for neighbor in node.neighbors:
            self.graph.remove_edge(node.index, neighbor.index)
        for neighbor in records[:self.view_size]:
            self.graph.add_edge(node.index, neighbor.index)
        node.set_neighbors(records[:self.view_size])

    def _merge_records(self, node: Node) -> List[Record]:
        buffer: List[Record] = copy.deepcopy(node.neighbors)
        node._pull_buffer = list(filter(self._local_filter(node),
                                        node._pull_buffer))
        node._pull_buffer = list(filter(self._dedup_filter(buffer),
                                        node._pull_buffer))

        buffer = list(filter(self._dedup_filter(node._pull_buffer), buffer))
        buffer = node._pull_buffer + buffer
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

    def partition_nodes(self, nodes: List[Node], partition_size: float) -> List[Node]:
        if self is PartitionType.Rand:
            return random.sample(nodes, k=int(len(nodes) * partition_size))
        elif self is PartitionType.Lineal:
            return nodes[:int(len(nodes) * partition_size)]
        raise ValueError("Invalid partition type")


def send_metrics(cluster: Cluster, run_id: str, tick: int):
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


def write_cluster(clusters: List[Cluster], run_id: str, tick: int, folder: str):
    folder = os.path.join(folder, run_id) if folder else run_id
    output_file = os.path.join(folder,f"{run_id}.{tick}.partition.sim")
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
@click.option("-f", "--fanout", type=int, default=1)
@click.option("-v", "--view-size", type=int, default=32)
@click.option("-sp", "--selection", type=str, default="rand")
@click.option("-pp", "--propagation", type=str, default="pushpull")
@click.option("-mp", "--merge", type=str, default="head")
@click.option("-pt", "--partition-tick", type=int)
@click.option("-ps", "--partition-size", type=float, default=0.5)
@click.option("-ptp", "--partition-type", type=str, default="rand")
@click.option("--influx", is_flag=True)
@click.option('-f', '--folder', help="Output folder to store the file to.",
              type=str)
@click.option('--plot', help="Plot simulation convergence graph.",
              is_flag=True)
def simulate(ticks: int, repetitions: int, step: int, fanout: int, min_nodes: int,
             max_nodes: int, view_size: int, selection: str, propagation: str,
             merge: str, partition_tick: int, partition_size: float, partition_type: str,
             influx: bool, folder: str, plot: bool):
    simulation_id = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    output_file = os.path.join(folder,
                               f"{simulation_id}.partition.pex.sim") if folder else f"{simulation_id}.partition.pex.sim"
    max_nodes = max(min_nodes, max_nodes)
    with open(output_file, "a") as file:
        file.write(f"{min_nodes} {max_nodes} {repetitions} {step}\n")
    init_metrics()
    partition_type = PartitionType.from_string(partition_type)
    for n in range(min_nodes, max_nodes + 1, step):
        for i in range(repetitions):
            global nodes
            nodes = [Node(node_id) for node_id in range(n)]
            clusters = []
            c0 = Cluster(fanout, view_size,
                         Policy.from_string(selection),
                         Policy.from_string(propagation),
                         Policy.from_string(merge))
            clusters.append(c0)
            c0.initialize_nodes(nodes)
            c0.initialize_topology(Topology.RING)

            run_id = "".join(random.choices(string.
                                            ascii_lowercase + string.digits, k=16))
            print(f"{n} - Run {run_id} ({i + 1}/{repetitions}) started")
            for tick in range(1,ticks+1):
                print(f"N={n}({i + 1}/{repetitions}) - Tick {tick + 1}/{ticks}...")
                if partition_tick and tick == partition_tick:
                    partition = partition_type.partition_nodes(list(c0.nodes.values()), partition_size)
                    clusters.append(c0.partition(partition))
                    print(f"Partitioned: {clusters}")
                for c in clusters:
                    c.simulate_tick(tick)
                    if influx:
                        send_metrics(c, run_id, tick)
                if not influx:
                    write_cluster(clusters, run_id, tick, folder)
            print(f"{n} - Run {run_id} ({i + 1}/{repetitions}) finished - {clusters}")

            with open(output_file, "a") as file:
                file.write(f"{os.path.join(folder, run_id)}\n")
    print(f"Results stored at {output_file}")

    if plot:
        subprocess.run(shlex.split(f"python3 convergence.py plot {output_file}"))


if __name__ == '__main__':
    simulate()
