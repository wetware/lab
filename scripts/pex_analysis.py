import itertools
import os
from statistics import mean
from typing import List
import matplotlib.pyplot as plt
import networkx as nx
from influxdb import InfluxDBClient
import click


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
@click.option("-pth", is_flag=True)
@click.option("-cc", is_flag=True)
@click.option("-mtx", is_flag=True)
@click.option("-all", is_flag=True)
@click.option("--influx", is_flag=True)
@click.option("-f", "--folder", type=str)
def plot(run: str, tick: List[int], dd: bool, pth: bool, cc: bool, mtx: bool, all: bool,
         influx: bool, folder: str):
    for i, t in enumerate(tick):
        if influx:
            graph = network_from_influx(run, t)
        else:
            graph = network_from_files(run, t, folder)
        if dd or all:
            degrees = [graph.degree(n) for n in graph.nodes()]
            plt.hist(degrees)
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
            plt.title(f"N={graph.number_of_nodes()}, Tick={t} - Adjacency matrix")
            plt.show()


@cli2.command()
@click.argument("runs", type=str, nargs=-1)
@click.option("-t", "--ticks", type=int, default=100)
@click.option("-cd", is_flag=True)
@click.option("-pth", is_flag=True)
@click.option("-cc", is_flag=True)
@click.option("-rd", is_flag=True)
@click.option("-pt", is_flag=True)
@click.option("-all", is_flag=True)
@click.option("--influx", is_flag=True)
@click.option("-f", "--folder", type=str)
def calculate(runs: str, ticks: int, cd: bool, pth: bool, cc: bool, rd: bool,
              pt: bool, all: bool, influx: bool, folder: str):
    if influx:
        n = network_from_influx(runs[0], 1).number_of_nodes()
    else:
        n = network_from_files(runs[0], 1, folder).number_of_nodes()

    CCs = [[] for _ in range(len(runs))]
    PTHs = [[] for _ in range(len(runs))]
    CDs = [[] for _ in range(len(runs))]
    RDs = [[] for _ in range(len(runs))]
    PTs = [[[] for _ in range(2)] for _ in range(len(runs))]
    INFOs = []
    tick = 0
    for i, run in enumerate(runs):
        if not influx:
            info = info_from_files(run, folder)
            INFOs.append(info)
        ccs = CCs[i]
        pths = PTHs[i]
        cds = CDs[i]
        rds = RDs[i]
        pts = PTs[i]

        for tick in range(1, ticks + 1):
            print(f"({run}) Calculating tick {tick}")
            if influx:
                g = network_from_influx(run, tick)
            else:
                g = network_from_files(run, tick, folder)

            if not g or g.number_of_nodes() == 0:
                break
            if cc or all:
                print(f"({run}) Tick {tick} - Calculating average clustering coefficient")
                ccs.append(nx.average_clustering(g))
            if pth or all:
                print(f"({run}) Tick {tick} - Calculating average shortest path length")
                pths.append(nx.average_shortest_path_length(g))
            if cd or all:
                print(f"({run}) Tick {tick} - Calculating average node degree")
                cds.append(mean([len(g.in_edges(n)) + len(g.out_edges(n)) for n in g.nodes]))
            if rd or all:
                print(f"({run}) Tick {tick} - Calculating average record degree")
                rds.append(mean([len(g.in_edges(n)) for n in g.nodes]))
            if pt or all:
                print(f"({run}) Tick {tick} - Calculating partition remember time")
                dead_links = [0, 0]
                for node in g.nodes:
                    for e in g.in_edges(node):
                        if g.nodes[e[0]]["cluster"] != g.nodes[e[1]]["cluster"]:
                            dead_links[int(g.nodes[node]["cluster"])] += 1
                pts[0].append(dead_links[0])
                pts[1].append(dead_links[1])

    if cc or all:
        for ccs, info in zip(CCs, INFOs):
            plt.plot(ccs, label=info)
        plt.legend()
        plt.title(f"N={n}, Tick={tick - 1} - Network clustering coefficient")
        plt.show()
    if pth or all:
        for pths, info in zip(PTHs, INFOs):
            plt.plot(pths, label=info)
        plt.legend()
        plt.title(f"N={n}, Tick={tick - 1} - Network average shortest path length")
        plt.show()
    if cd or all:
        for cds, info in zip(CDs, INFOs):
            plt.plot(cds, label=info)
        plt.legend()
        plt.title(f"N={n}, Tick={tick - 1} - Network average node degree")
        plt.show()
    if rd or all:
        for rds, info in zip(RDs, INFOs):
            plt.plot(rds, label=info)
        plt.legend()
        plt.title(f"N={n}, Tick={tick - 1} - Network average records")
        plt.show()
    if pt or all:
        g = network_from_files(runs[0], tick - 1, folder)
        partitions = [0, 0]
        for node in g.nodes:
            partitions[g.nodes[node]["cluster"]] += 1

        partition_tick = next((i for i, x in enumerate(PTs[0][0]) if x), None)
        dmax = max(sorted([d for n, d in g.out_degree()], reverse=True))

        plt.ylabel("proportion of deadlinks")
        plt.xlabel("ticks")

        for pts, info in zip(PTs, INFOs):
            plt.plot([n / (partitions[0] * dmax) for n in pts[0][partition_tick:]], label=info)
        plt.legend()
        plt.title(
            f"N={n}, Tick={tick - 1}, Partition={(partitions[0]) / g.number_of_nodes()} - Network partition remember time")
        plt.show()

        for pts, info in zip(PTs, INFOs):
            plt.plot([n / (partitions[1] * dmax) for n in pts[1][partition_tick:]], label=info)
        plt.legend()
        plt.title(
            f"N={n}, Tick={tick - 1}, Partition={(partitions[1]) / g.number_of_nodes()} - Network partition remember time")
        plt.show()





def network_from_influx(run: str, tick: int) -> nx.DiGraph:
    graph = nx.DiGraph()
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")

    nodes = {}
    nodes_seq = {}
    for i, point in enumerate(client.query(
            f'''SHOW TAG VALUES from "diagnostics.casm-pex-convergence.view.point" WITH key=node WHERE run='{run}' ''').get_points()):
        nodes[i] = 0
        nodes_seq[point["value"]] = i

    # Extract node views at tick 'tick'
    results = client.query(f'''SELECT * FROM "diagnostics.casm-pex-convergence.view.point" WHERE run='{run}' ''')
    found = set()
    for point in results.get_points(tags={"tick": str(tick)}):
        if not point["records"]:
            continue
        if point["node"] in found:
            continue
        else:
            found.add(point["node"])

        for record in point["records"].split("-"):
            if record:
                graph.add_edge(nodes_seq[point["node"]], nodes_seq[record])
        nx.set_node_attributes(graph, {nodes_seq[point["node"]]: point["cluster"]}, name="cluster")
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


cli = click.CommandCollection(sources=[cli1, cli2, cli3])

if __name__ == '__main__':
    cli()
