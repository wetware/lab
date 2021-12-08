import numpy as np
import networkx as nx
import os.path
import random
import string
from datetime import datetime
from enum import IntEnum, auto
from typing import List, Dict

import click
import simpy
from influxdb import InfluxDBClient

nodes: List["Node"] = []
env = simpy.Environment()


class Topology(IntEnum):
    RING = auto()
    RAND = auto()
    INTERNET = auto()

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
        self.on = True
        self.cluster: Cluster = None

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
        self.cluster = cluster

    def select_neighbors(self, fanout: int):
        return random.sample(self.neighbors, k=fanout)


class Cluster:
    next_id = 0

    def __init__(self, fanout: int, c: int, S: int, R: int, X: float):
        self.fanout = fanout
        self.c = c
        self.S = S
        self.R = R
        self.D = X
        self.overlay = nx.DiGraph()
        self.underlay = None
        self.nodes: Dict[int, Node] = {}
        self.tick = 0
        self.id = Cluster.next_id
        Cluster.next_id += 1

        self.on = True

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

    def initialize_underlay(self, topology: Topology, distance_delay: float):
        self.underlay = nx.random_internet_as_graph(len(self.nodes))
        self._delays = {}
        for node, lengths in nx.all_pairs_shortest_path_length(self.underlay):
            for src in lengths:
                lengths[src] *= distance_delay
            self._delays[node] = lengths

    def partition(self, nodes: List[Node]) -> "Cluster":
        partition = Cluster(self.fanout, self.c, self.S, self.R,
                            self.D)
        partition.overlay = self.overlay
        partition.initialize_nodes(nodes)
        for node in nodes:
            self.nodes.pop(node.index)
        return partition

    def simulate_node(self, node: Node, sleep: float):
        while node.cluster.on:
            yield env.timeout(sleep)
            for record in node.select_neighbors(self.fanout):
                neighbor = Node.from_record(record)
                if not neighbor:
                    continue
                # Pushpull
                env.process(self._pushpull(node, neighbor))

    def _pushpull(self, node: Node, neighbor: Node):
        records = self._push(node, neighbor)
        yield env.timeout(self._delays[node.index][neighbor.index])
        self._pull(neighbor, records)
        records = self._push(neighbor, node)
        yield env.timeout(self._delays[neighbor.index][node.index])
        self._pull(node, records)

    def _push(self, node: Node, neighbor: Node):
        sorted_records = sorted(node.neighbors, key=lambda r: r.hop, reverse=True)
        R = min(self.R, len(sorted_records))
        youngest, oldest = sorted_records[R:], sorted_records[:R]
        np.random.shuffle(youngest)
        node.neighbors = youngest + oldest
        c = min(self.c // 2, len(node.neighbors))
        buffer = [n.copy() for n in node.neighbors[:c]]
        buffer.append(node.record)
        return buffer

    def _pull(self, node: Node, records: List[Record]):
        records = self._merge_records(node, records)
        S = min(self.S, max(len(records) - self.c, 0))
        records = records[S:]

        R = min(self.R, len(records), self.c)

        records = sorted(records, key=lambda r: r.hop, reverse=True)
        oldest, records = records[:R], records[R:]
        np.random.shuffle(records)
        np.random.shuffle(oldest)
        while len(oldest) + len(records) > self.c and oldest and random.random() < self.D:
            oldest = oldest[1:]
        c = self.c - len(oldest)
        records = records[:c] + oldest

        node._pull_buffer = []

        s_old, s_new = set(map(lambda n: n.index, node.neighbors)), set(map(lambda n: n.index, records))
        for i in s_old - s_new:
            self.overlay.remove_edge(node.index, i)
        for i in s_new - s_old:
            self.overlay.add_edge(node.index, i)

        node.neighbors = records
        for neighbor in node.neighbors:
            neighbor.hop += 1

    def _merge_records(self, node: Node, pull_records: List[Record]) -> List[Record]:
        buffer: List[Record] = []
        pull_set = dict((r.index, r) for r in pull_records)
        for r1 in node.neighbors:
            r2 = pull_set.get(r1.index, None)
            if not r2 or r1.hop <= r2.hop:
                buffer.append(r1)

        buffer_set = dict((r.index, r) for r in buffer)
        for r1 in pull_records:
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
                "cluster": node.cluster.id
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
@click.option("-R", type=int, default=0)
@click.option("-D", type=float, default=0.5)
@click.option("-dd", "--distance-delay", type=float, default=0.1)
@click.option("-sl", "--sleep", type=float, default=1)
@click.option("-p", "--partition", multiple=True, type=str)
@click.option("-sp", "--simulate-partition", is_flag=True)
@click.option("--influx", is_flag=True)
@click.option('-f', '--folder', help="Output folder to store the file to.",
              type=str)
def simulate(ticks: int, repetitions: int, nodes_amount: int, fanout: int,
             c: int, s: int, r: int, d: float, distance_delay: float, sleep: float,
             partition: List[str], simulate_partition: bool, influx: bool, folder: str):
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
        write_info_to_file(run_id, {"S": s, "R": r, "D": d, "c": c}, folder)

        c0 = Cluster(fanout, c, s, r, d)
        nodes = [Node(node_id) for node_id in range(nodes_amount)]
        c0.initialize_nodes(nodes)
        c0.initialize_overlay(Topology.RING)
        c0.initialize_underlay(Topology.INTERNET, distance_delay)

        for node in nodes:
            env.process(c0.simulate_node(node, sleep))

        def manage_simulation():
            print(f"{nodes_amount} - Run {run_id} ({rep + 1}/{repetitions}) started")
            for tick in range(1, ticks + 1):
                yield env.timeout(sleep)
                print(f"N={nodes_amount}({rep + 1}/{repetitions}) - Tick {tick}/{ticks}...")
                for partition_tick, partition_size in partitions:
                    if partition_tick and tick == partition_tick:
                        pnodes = partition_nodes(partition_type, list(c0.nodes.values()), partition_size)
                        c = c0.partition(pnodes)
                        c.on = False
                        print(f"Partitioned: {c0}")

                if influx:
                    write_to_influx(c0, run_id, tick)
                else:
                    write_to_file(c0, run_id, tick, folder)
            print(f"{nodes_amount} - Run {run_id} ({rep + 1}/{repetitions}) finished - {c0}")
            with open(output_file, "a") as file:
                file.write(f"{os.path.join(folder, run_id)}\n")
            c0.on = False

        env.process(manage_simulation())
        env.run()
    print(f"Results stored at {output_file}")


if __name__ == '__main__':
    simulate()
