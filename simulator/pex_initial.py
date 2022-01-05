import numpy as np
import networkx as nx
import os.path
import random
import string
from datetime import datetime
from enum import IntEnum, auto
from typing import List, Dict

import click
from influxdb import InfluxDBClient

nodes: List["Node"] = []


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


def are_neighbors(topology, i: int, j: int, nodes_amount: int):
    if topology == Topology.RING:
        if i + 1 == j or j + 1 == i:
            return True
        if i == 0 and j == nodes_amount - 1:
            return True
        if j == 0 and i == nodes_amount - 1:
            return True
    if topology is Topology.RAND:
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

    def set_cluster(self, cluster: "Cluster"):
        self._cluster = cluster

    def select_neighbors_rand(self, fanout: int):
        return random.sample(self.neighbors, k=fanout)

    def select_neighbors_tail(self, fanout: int):
        return sorted(self.neighbors, key= lambda r: r.hop, reverse=True)[:fanout]


class Cluster:
    next_id = 0

    def __init__(self, fanout: int, c: int, S: int, H: int, X: float, tail: bool):
        self.fanout = fanout
        self.c = c
        self.S = S
        self.H = H
        self.tail = tail
        self.D = X
        self.overlay = nx.DiGraph()
        self.nodes: Dict[int, Node] = {}
        self.tick = 0
        self.id = Cluster.next_id
        Cluster.next_id += 1

    def initialize_nodes(self, nodes: List[Node]):
        for node in nodes:
            self.nodes[node.index] = node
            node.set_cluster(self)
            if self.overlay.has_node(node.index):
                self.overlay.nodes[node.index]["cluster"] = self.id
            else:
                self.overlay.add_node(node.index, cluster=self.id)

    def initialize_overlay(self, topology: Topology):
        for i, node in self.nodes.items():
            for j, neighbor in self.nodes.items():
                if i == j:
                    continue
                if are_neighbors(topology, i, j, len(self.nodes)):
                    record = neighbor.record
                    record.hop += 1
                    node.neighbors.append(record)
                    self.overlay.add_edge(node.index, neighbor.index)


    def partition(self, nodes: List[Node]) -> "Cluster":
        partition = Cluster(self.fanout, self.c, self.S, self.H,
                            self.D, self.tail)
        partition.overlay = self.overlay
        partition.initialize_nodes(nodes)
        for node in nodes:
            self.nodes.pop(node.index)
        return partition

    def simulate_tick(self, i: int):
        for node in self.nodes.values():
            neighbor_records = node.select_neighbors_tail(self.fanout) if self.tail else node.select_neighbors_rand(self.fanout)
            for record in neighbor_records:
                neighbor = Node.from_record(record)
                if not neighbor:
                    continue
                # Pushpull
                self._push(node, neighbor)
                self._push(neighbor, node)
                self._pull(node, neighbor)
                self._pull(neighbor, node)
        self.tick += 1

    def _push(self, node: Node, neighbor: Node):
        sorted_records = sorted(node.neighbors, key=lambda r: r.hop, reverse=True)
        R = min(self.H, len(sorted_records))
        youngest, oldest = sorted_records[R:], sorted_records[:R]
        np.random.shuffle(youngest)
        node.neighbors = youngest + oldest
        c = min(self.c // 2, len(node.neighbors))
        neighbor._pull_buffer = [n.copy() for n in node.neighbors[:c]]
        neighbor._pull_buffer.append(node.record)

    def _pull(self, node: Node, neighbor: Node):
        records = self._merge_records(node)
        S = min(self.S, max(len(records) - self.c, 0))
        records = records[S:]

        H = min(self.H, max(len(records) - self.c, 0))

        records = sorted(records, key=lambda r: r.hop, reverse=True)
        records = records[H:]
        np.random.shuffle(records)
        records = records[:self.c]

        node._pull_buffer = []

        s_old, s_new = set(map(lambda n: n.index, node.neighbors)), set(map(lambda n: n.index, records))
        for i in s_old - s_new:
            self.overlay.remove_edge(node.index, i)
        for i in s_new - s_old:
            self.overlay.add_edge(node.index, i)

        node.neighbors = records
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
    RAND = auto()
    LINEAL = auto()

    @staticmethod
    def from_string(name: str):
        if name == "rand":
            return PartitionType.RAND
        if name == "lineal":
            return PartitionType.LINEAL
        raise ValueError("Invalid partition type name")


def partition_nodes(partition_type, nodes: List[Node], partition_size: int) -> List[Node]:
    if partition_type is PartitionType.RAND:
        return random.sample(nodes, k=min(len(nodes), partition_size))
    elif partition_type is PartitionType.LINEAL:
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


def write_to_file(cluster: Cluster, run_id: str, tick: int, folder: str):
    folder = os.path.join(folder, run_id) if folder else run_id
    output_file = os.path.join(folder, f"{run_id}.{tick}.partition.sim")
    if not os.path.isdir(folder):
        os.mkdir(folder)

    nx.write_gpickle(cluster.overlay, output_file)


@click.command()
@click.option("-t", "--ticks", type=int, default=50)
@click.option("-r", "--repetitions", type=int, default=1)
@click.option("-n", "--nodes-amount", type=int, default=3)
@click.option("-f", "--fanout", type=int, default=1)
@click.option("-c", type=int, default=32)
@click.option("-S", type=int, default=0)
@click.option("-H", type=int, default=0)
@click.option("-T", "--tail", is_flag=True)
@click.option("-D", type=float, default=0.5)
@click.option("-p", "--partition", multiple=True, type=str)
@click.option("--influx", is_flag=True)
@click.option('-f', '--folder', help="Output folder to store the file to.",
              type=str)
def simulate(ticks: int, repetitions: int, nodes_amount: int, fanout: int,
             c: int, s: int, h: int, tail, d: float, partition: List[str],
             influx: bool, folder: str):
    simulation_id = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    output_file = os.path.join(folder,
                               f"{simulation_id}.partition.pex.sim") if folder else f"{simulation_id}.partition.pex.sim"
    with open(output_file, "a") as file:
        file.write(f"{nodes_amount} {repetitions} \n")
    partitions = [(int(p.split(":")[0]), int(p.split(":")[1])) for p in partition]
    partition_type = PartitionType.RAND

    for rep in range(repetitions):
        global nodes
        run_id = "".join(random.choices(string.
                                        ascii_lowercase + string.digits, k=16))
        write_info_to_file(run_id, {"S": s, "R": h, "D": d, "c": c, "tail": tail}, folder)

        c0 = Cluster(fanout, c, s, h, d, tail)
        nodes = [Node(node_id) for node_id in range(nodes_amount)]
        c0.initialize_nodes(nodes)
        c0.initialize_overlay(Topology.RING)

        print(f"{nodes_amount} - Run {run_id} ({rep + 1}/{repetitions}) started")
        for tick in range(1, ticks + 1):
            print(f"N={nodes_amount}({rep + 1}/{repetitions}) - Tick {tick}/{ticks}...")
            for partition_tick, partition_size in partitions:
                if partition_tick and tick == partition_tick:
                    pnodes = partition_nodes(partition_type, list(c0.nodes.values()), partition_size)
                    c0.partition(pnodes)
                    print(f"Partitioned: {c0}")

            c0.simulate_tick(tick)
            if influx:
                write_to_influx(c0, run_id, tick)
            if not influx:
                write_to_file(c0, run_id, tick, folder)
        print(f"{nodes_amount} - Run {run_id} ({rep + 1}/{repetitions}) finished - {c0}")
        with open(output_file, "a") as file:
            file.write(f"{os.path.join(folder, run_id)}\n")
    print(f"Results stored at {output_file}")


if __name__ == '__main__':
    simulate()
