import copy
import os.path
import random
import shlex
import string
import subprocess
from datetime import datetime
from enum import IntEnum, auto
from typing import List

import click
from influxdb import InfluxDBClient

import convergence


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
        self.tick = 0

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
        self.tick += 1

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


def send_metrics(cluster: Cluster, run_id: str):
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")
    json_body = []
    for node in cluster.nodes:
        point = {
            "measurement": "diagnostics.casm-pex-convergence.view.point",
            "tags": {
                "node": str(node.index),
                "records": "-".join(map(lambda r: str(r.index), node.neighbors)),
                "tick": str(cluster.tick),
                "run": run_id
            },
            "time": datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ'),
            "fields": {
                "value": 0.0
            }
        }
        json_body.append(point)
    client.write_points(json_body)


def init_metrics():
    subprocess.run(shlex.split("docker start testground-influxdb"))


@click.command()
@click.option("-t", "--ticks", type=int, default=30)
@click.option("-r", "--repetitions", type=int, default=1)
@click.option("-s", "--step", type=int, default=1)
@click.option("-min", "--min-nodes", type=int, default=3)
@click.option("-max", "--max-nodes", type=int, default=3)
@click.option("-f", "--fanout", type=int, default=1)
@click.option("-v", "--view-size", type=int, default=32)
@click.option("-sp", "--selection", type=str, default="rand")
@click.option("-pp", "--propagation", type=str, default="pushpull")
@click.option("-mp", "--merge", type=str, default="head")
@click.option('-f', '--folder', help="Output folder to store the file to.",
              type=str)
@click.option('--plot', help="Plot simulation convergence graph.",
              is_flag=True)
def simulate(ticks: int, repetitions: int, step: int, fanout: int, min_nodes: int,
             max_nodes: int, view_size: int, selection: str, propagation: str,
             merge: str, folder: str, plot: bool):
    simulation_id = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    output_file = os.path.join(folder, f"{simulation_id}-pex.sim") if folder else f"{simulation_id}-pex.sim"
    with open(output_file, "a") as file:
        file.write(f"{min_nodes} {max_nodes} {repetitions} {step}\n")
    init_metrics()
    for n in range(min_nodes, max_nodes + 1, step):
        for i in range(repetitions):
            cluster = Cluster(fanout, view_size,
                              Policy.from_string(selection),
                              Policy.from_string(propagation),
                              Policy.from_string(merge))
            cluster.initialize_nodes(n)
            cluster.initialize_topology(Topology.RING)

            run_id = "".join(random.choices(string.
                                            ascii_lowercase + string.digits, k=16))
            print(f"{n} - Run {run_id} ({i + 1}/{repetitions}) started")
            for j in range(ticks):
                print(f"{n}({i+1}/{repetitions}) - Tick {j + 1}/{ticks}...")
                cluster.simulate_tick(j)
                send_metrics(cluster, run_id)
            print(f"{n} - Run {run_id} ({i + 1}/{repetitions}) finished")

            with open(output_file, "a") as file:
                file.write(f"{os.path.join(folder, run_id)}\n")
    print(f"Results stored at {output_file}")

    if plot:
        subprocess.run(shlex.split(f"python3 convergence.py plot {output_file}"))


if __name__ == '__main__':
    simulate()
