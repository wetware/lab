import os
from statistics import mean
from typing import List
import matplotlib.pyplot as plt
import networkx as nx
import numpy as np
from influxdb import InfluxDBClient
import click

plt.rcParams.update({"font.size": 22})


@click.group()
def cli1():
    pass


@click.group()
def cli2():
    pass


@click.group()
def cli3():
    pass


@cli1.command()
@click.argument("run", type=str)
@click.option("-t", "--tick", type=int, multiple=True, default=[1, 100])
@click.option("-dd", is_flag=True)
@click.option("-dd", is_flag=True)
@click.option("-pth", is_flag=True)
@click.option("-cc", is_flag=True)
@click.option("-mtx", is_flag=True)
@click.option("-pt", is_flag=True)
@click.option("-all", is_flag=True)
@click.option("-r", "--repetitions", type=int, default=2)
@click.option("--influx", is_flag=True)
@click.option("-f", "--folder", type=str)
def static(run: str, tick: List[int], dd: bool, pth: bool, cc: bool, mtx: bool,
           pt: bool, all: bool, repetitions: int, influx: bool, folder: str):
    for i, t in enumerate(tick):
        if influx:
            graph = network_from_influx(run, t)
            info = info_from_influx(run, folder)
        else:
            graph = network_from_files(run, t, folder)
            info = info_from_files(run, folder)
        if dd or all:
            degrees = [graph.degree(n) for n in graph.nodes()]
            plt.hist(degrees)
            plt.ylabel("nodes amount")
            plt.xlabel("degree")
            plt.title(f"N={graph.number_of_nodes()}, Tick={t} - Degree distribution")
            plt.show()
        if cc or all:
            coefficients = [cc for cc in nx.clustering(graph).values()]
            plt.hist(coefficients)
            plt.title(f"N={graph.number_of_nodes()}, Tick={t} - Clustering coefficient distribution")
            plt.show()
        if pth or all:
            avg_shortest_paths = [mean(lengths.values()) for scr, lengths in nx.all_pairs_shortest_path_length(graph)]
            plt.hist(avg_shortest_paths)
            plt.title(f"N={graph.number_of_nodes()}, Tick={t} - Average shortest path length distribution")
            plt.show()
        if mtx or all:
            matrix = nx.to_numpy_matrix(graph)
            plt.matshow(matrix, cmap=plt.cm.Blues, fignum=i)
            plt.title(f"N={graph.number_of_nodes()} - Adjacency matrix")
            plt.show()
        if pt or all:
            y = []
            for p in range(1, 100):
                print(f"Calculating if there is partition evicting {p}% of the nodes")
                avgs = []
                for _ in range(repetitions):
                    g2 = graph.to_undirected()
                    evict_nodes = np.random.choice(g2.nodes, int(g2.number_of_nodes() * (p / 100)), replace=False)
                    g2.remove_nodes_from(evict_nodes)
                    partition_lens = [len(c) for c in sorted(nx.connected_components(g2), key=len, reverse=True)]
                    if len(partition_lens) > 1:
                        avgs.append(sum(partition_lens[1:]) / graph.number_of_nodes())
                    else:
                        avgs.append(0)
                y.append(mean(avgs))
            plt.ylabel("Proportion of nodes outside of largest cluster")
            plt.xlabel("Evicted % of nodes")
            plt.plot(y, label=info)
            plt.legend()
            plt.xticks([step for step in range(0, 105, 5)])
            plt.title(f"N={graph.number_of_nodes()}, Tick={t} - Partition resistance")
            plt.show()


@cli2.command()
@click.argument("runs", type=str, nargs=-1)
@click.option("-t", "--ticks", type=int, default=100)
@click.option("-c", is_flag=True)
@click.option("-nd", is_flag=True)
@click.option("-pth", is_flag=True)
@click.option("-cc", is_flag=True)
@click.option("-nid", is_flag=True)
@click.option("-pt", is_flag=True)
@click.option("-ptb", is_flag=True)
@click.option("-ptr", is_flag=True)
@click.option("-r", "--repetitions", type=int, default=2)
@click.option("-all", is_flag=True)
@click.option("--influx", is_flag=True)
@click.option("-f", "--folder", type=str)
def dynamic(runs: str, ticks: int, c: bool, nd: bool, pth: bool, cc: bool, nid: bool,
            pt: bool, ptb: bool, ptr: bool, repetitions: int, all: bool, influx: bool, folder: str):
    CCs = [[] for _ in range(len(runs))]
    PTHs = [[] for _ in range(len(runs))]
    CDs = [[] for _ in range(len(runs))]
    RDs = [[] for _ in range(len(runs))]
    PTs = [[] for _ in range(len(runs))]
    PTRs = [[] for _ in range(len(runs))]
    Ns = [[] for _ in range(len(runs))]
    INFOs = []
    tick = 0
    for i, run in enumerate(runs):
        print(i, run)
        n, partitions_n = size_and_partitions(folder, influx, run, ticks)
        if not influx:
            info = info_from_files(run, folder)
            INFOs.append(info)
        else:
            info = info_from_influx(run, folder)
            INFOs.append(info)
        ccs = CCs[i]
        pths = PTHs[i]
        cds = CDs[i]
        rds = RDs[i]
        pts = PTs[i]
        ns = Ns[i]
        for _ in range(partitions_n):
            pts.append([])

        for tick in range(1, ticks + 1):
            if not any([nd, pth, cc, nid, pt, ptb, c]):
                continue
            print(f"({run}) Calculating tick {tick}")
            if influx:
                g = network_from_influx(run, tick)
            else:
                g = network_from_files(run, tick, folder)
            if not g or g.number_of_nodes() == 0:
                tick -= 1
                break
            if c or all:
                print(f"({run}) Tick {tick} - Is connected: {nx.is_weakly_connected(g)}")
            if cc or all:
                print(f"({run}) Tick {tick} - Calculating average clustering coefficient")
                ccs.append(nx.average_clustering(g))
            if pth or all:
                print(f"({run}) Tick {tick} - Calculating average shortest path length")
                try:
                    pths.append(nx.average_shortest_path_length(g))
                except nx.NetworkXError:
                    pass
            if nd or all:
                print(f"({run}) Tick {tick} - Calculating average node degree")
                cds.append(mean([len(g.in_edges(n)) + len(g.out_edges(n)) for n in g.nodes]))
            if nid or all:
                print(f"({run}) Tick {tick} - Calculating average node indegree")
                rds.append(mean([len(g.in_edges(n)) for n in g.nodes]))
            if pt or ptb or all:
                print(f"({run}) Tick {tick} - Calculating partition remember time")
                N = 0
                dead_links = [0 for _ in range(partitions_n)]
                for node, data in g.nodes(data=True):
                    for e in g.in_edges(node):
                        p1, p2 = int(g.nodes[e[0]]["cluster"]), int(g.nodes[e[1]]["cluster"])
                        if p1 == 0 and p2 != 0:
                            dead_links[p2] += 1
                    if int(data["cluster"]) == 0:
                        N += 1
                for j, deadlink in enumerate(dead_links):
                    pts[j].append(deadlink)
                ns.append(N)

    if cc or all:
        for ccs, info in zip(CCs, INFOs):
            plt.plot(ccs, label=info)
        plt.legend()
        plt.ylabel("clustering coefficient")
        plt.xlabel("tick")
        # plt.title(f"N={n}, Ticks={tick} - Network clustering coefficient")
        plt.show()
    if pth or all:
        for pths, info in zip(PTHs, INFOs):
            plt.plot(pths, label=info)
        plt.legend()
        plt.ylabel("average path length")
        plt.xlabel("tick")
        # plt.title(f"N={n}, Ticks={tick} - Network average shortest path length")
        plt.show()
    if nd or all:
        for cds, info in zip(CDs, INFOs):
            plt.plot(cds, label=info)
        plt.legend()
        plt.ylabel("average node degree")
        plt.xlabel("tick")
        # plt.title(f"N={n}, Ticks={tick} - Network average node degree")
        plt.show()
    if nid or all:
        for rds, info in zip(RDs, INFOs):
            plt.plot(rds, label=info)
        plt.legend()
        plt.ylabel("average node indegree")
        plt.xlabel("tick")
        # plt.title(f"N={n}, Tick={tick} - Network average node indegree")
        plt.show()
    if pt or all:
        if influx:
            g = network_from_influx(runs[0], tick)
        else:
            g = network_from_files(runs[0], tick, folder)
        partitions = []
        for node in g.nodes:
            p = int(g.nodes[node]["cluster"])
            while len(partitions) <= p:
                partitions.append(0)
            partitions[p] += 1

        partition_tick = next((i for i, x in enumerate(PTs[0][1]) if x), None)
        dmax = max(sorted([d for _, d in g.out_degree()], reverse=True))


        plt.ylabel("proportion of deadlinks")
        plt.xlabel("ticks")

        for pts, info in zip(PTs, INFOs):
            pt = [sum(links) for links in zip(*pts)]
            plt.plot([deadlinks_n / (partitions[0] * dmax) for deadlinks_n in pt[partition_tick:]], label=info)
        plt.legend()
        # plt.title(
        #   f"N={n}, Ticks={tick}, Partition={(partitions[0]) / g.number_of_nodes()} - Network partition remember time")
        plt.show()

    if ptb or all:
        if influx:
            g = network_from_influx(runs[0], tick)
        else:
            g = network_from_files(runs[0], tick, folder)
        partitions = []
        for node in g.nodes:
            p = int(g.nodes[node]["cluster"])
            while len(partitions) <= p:
                partitions.append(0)
            partitions[p] += 1

        partition_tick = next((i for i, x in enumerate(PTs[0][1]) if x), None)
        dmax = max(sorted([d for _, d in g.out_degree()], reverse=True))

        plt.ylabel("proportion of deadlinks")
        plt.xlabel("ticks")

        pi = 0
        for pts, info in zip(PTs, INFOs):
            x = [i for i in range(1, tick - partition_tick + 1)]

            ys = []
            ns = Ns[pi]
            pi += 1
            for i, pt in enumerate(pts[1:], start=1):
                y = [deadlinks_n / (ns[di] * dmax) for di, deadlinks_n in
                     enumerate(pt[partition_tick:], start=partition_tick)]
                bottom = [sum(bottom) for bottom in zip(*ys)] if ys else None
                ys.append(y)
                label = f"Partition {i}. {info}"
                plt.bar(x, y, bottom=bottom, label=label)
        plt.legend()
        # plt.title(
        #   f"N={n}, Ticks={tick}, Partition={(partitions[0]) / g.number_of_nodes()} - Network partition remember time")
        plt.show()
    if ptr or all:
        for ri, run in enumerate(runs):
            if influx:
                g = network_from_influx(runs[0], tick)
                info = info_from_influx(runs[0], folder)
            else:
                g = network_from_files(runs[0], tick, folder)
                info = info_from_files(run, folder)
            ptr = PTRs[ri]
            for p in range(1, 70):
                ptr.append(0)
            for p in range(70, 100):
                print(f"({run}) Calculating if there is partition evicting {p}% of the nodes")
                avgs = []
                for _ in range(repetitions):
                    g2 = g.to_undirected()
                    evict_nodes = np.random.choice(g2.nodes, int(g2.number_of_nodes() * (p / 100)), replace=False)
                    g2.remove_nodes_from(evict_nodes)
                    partition_lens = [len(c) for c in sorted(nx.connected_components(g2), key=len, reverse=True)]
                    if len(partition_lens) > 1:
                        avgs.append((sum(partition_lens[1:]) / g.number_of_nodes()) * 100)
                    else:
                        avgs.append(0)
                ptr.append(mean(avgs))
            plt.plot(ptr, label=info)
        plt.ylabel("% of partitioned nodes")
        plt.xlabel("Evicted % of nodes")
        plt.legend()
        plt.xticks([step for step in range(0, 105, 5)])
        # plt.title(f"N={g.number_of_nodes()}, Ticks={tick} - Partition resistance")
        plt.show()


def size_and_partitions(folder, influx, run, ticks):
    if influx:
        g = None
        t = ticks
        while not g:
            print(run, t)
            g = network_from_influx(run, t)
            t -= 1
    else:
        g = None
        t = ticks
        while not g:
            g = network_from_files(run, t, folder)
            t -= 1
    n = g.number_of_nodes()
    partitions_n = len(set(nx.get_node_attributes(g, "cluster").values()))
    return n, 2


def network_from_influx(run: str, tick: int) -> nx.DiGraph:
    graph = nx.DiGraph()
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")

    max_tick = max(map(lambda t: int(t["value"]), client.query(
        f'''SHOW TAG VALUES from "view" WITH key=tick WHERE run='{run}' ''').get_points()))
    if max_tick < tick:
        return graph

    nodes = {}
    nodes_seq = {}

    for i, point in enumerate(client.query(
            f'''SHOW TAG VALUES from "view" WITH key=node WHERE run='{run}' ''').get_points()):
        nodes[i] = 0
        nodes_seq[point["value"]] = i

    # Extract node views at tick 'tick'
    results = client.query(
        f'''SELECT * FROM "view" WHERE run='{run}' AND tick='{tick}' ''')
    found = set()
    all_nodes = set()
    for point in results.get_points():
        if not point["records"]:
            continue
        node = point["node"]
        node_seq = nodes_seq[node]
        if node in found:
            continue
        else:
            found.add(node)

        records = point["records"].split("-")

        for record in records:
            if record:
                graph.add_edge(node_seq, nodes_seq[record])
                all_nodes.add(record)
        nx.set_node_attributes(graph, {node_seq: point.get("cluster", 0)}, name="cluster")
    for node in all_nodes - found:
        nx.set_node_attributes(graph, {nodes_seq[node]: 1}, name="cluster")

    return graph


def network_from_files(run: str, tick: int, folder: str) -> nx.DiGraph:
    folder = os.path.join(folder, run) if folder else run
    input_file = os.path.join(folder, f"{run}.{tick}.partition.sim")
    if os.path.isfile(input_file):
        return nx.read_gpickle(input_file)
    return None


def info_from_files(run: str, folder: str):
    folder = os.path.join(folder, run) if folder else run
    input_file = os.path.join(folder, "info.sim")
    if os.path.isfile(input_file):
        with open(input_file) as file:
            params = ", ".join(file.read().splitlines())
            # params = dict(tuple(line.split("=")) for line in file.readlines())
            return params
    return None


def info_from_influx(run: str, folder: str):
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")
    results = client.query(f'''SELECT * FROM "view" WHERE run='{run}' ''')
    point = next(results.get_points(tags={"tick": str(1)}))
    return f"C={point['C']}, S={point['S']}, R={point['R']}, D={point['D']},"


cli = click.CommandCollection(sources=[cli1, cli2, cli3])

if __name__ == '__main__':
    cli()
