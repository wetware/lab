import itertools
from statistics import mean
from typing import List
import matplotlib.pyplot as plt
import networkx as nx
import numpy as np
from influxdb import InfluxDBClient
import click
from matplotlib.ticker import MaxNLocator


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
def plot(run: str, tick: List[int], dd: bool, pth: bool, cc: bool, mtx: bool, all: bool):
    for t in tick:
        graph = network(run, t)
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
            plt.matshow(matrix, cmap=plt.cm.Blues)
            plt.title(f"N={graph.number_of_nodes()}, Tick={t} - Adjacency matrix")
            plt.show()


@cli2.command()
@click.argument("run", type=str)
@click.option("-t", "--ticks", type=int, default=100)
@click.option("-cd", is_flag=True)
@click.option("-pth", is_flag=True)
@click.option("-cc", is_flag=True)
@click.option("-rd", is_flag=True)
@click.option("-pt", is_flag=True)
@click.option("-all", is_flag=True)
def calculate(run: str, ticks: int, cd: bool, pth: bool, cc: bool, rd: bool, pt: bool, all: bool):
    n = network(run, 1).number_of_nodes()

    ccs = []
    pths = []
    cds = []
    rds = []
    pts = [[0 for _ in range(n)] for _ in range(2)]

    for t in range(1, ticks + 1):
        print(f"Calculating tick {t}")
        g = network(run, t)
        if g.number_of_nodes() == 0:
            break
        if cc or all:
            print(f"Tick {t} - Calculating average clustering coefficient")
            ccs.append(nx.average_clustering(g))
        if pth or all:
            print(f"Tick {t} - Calculating average shortest path length")
            pths.append(nx.average_shortest_path_length(g))
        if cd or all:
            print(f"Tick {t} - Calculating average node degree")
            cds.append(mean([len(g.in_edges(n)) + len(g.out_edges(n)) for n in g.nodes]))
        if rd or all:
            print(f"Tick {t} - Calculating average record degree")
            rds.append(mean([len(g.in_edges(n)) for n in g.nodes]))
        if pt or all:
            print(f"Tick {t} - Calculating partition remember time")
            for node in g.nodes:
                for e in g.in_edges(node):
                    if g.nodes[e[0]]["cluster"] != g.nodes[e[1]]["cluster"]:
                        pts[int(g.nodes[node]["cluster"])][node] += 1
                        break

    if cc or all:
        plt.plot(ccs)
        plt.title(f"N={n}, Tick={ticks} - Network clustering coefficient")
        plt.show()
    if pth or all:
        plt.plot(pths)
        plt.title(f"N={n}, Tick={ticks} - Network average shortest path length")
        plt.show()
    if cd or all:
        plt.plot(cds)
        plt.title(f"N={n}, Tick={ticks} - Network average node degree")
        plt.show()
    if rd or all:
        plt.plot(rds)
        plt.title(f"N={n}, Tick={ticks} - Network average records")
        plt.show()
    if pt or all:
        pts_chained = list(itertools.chain(*[list(filter(lambda n: n != 0, pt)) for pt in pts]))
        plt.hist(pts_chained)
        plt.title(f"N={n}, Tick={ticks} - Network partition remember time")
        plt.show()

        pts = [mean(filter(lambda n: n != 0, pt)) for pt in pts]
        plt.bar([0, 1], pts, tick_label=[0, 1])
        plt.title(f"N={n}, Tick={ticks} - Network partition remember time")
        plt.show()


def network(run: str, tick: int) -> nx.DiGraph:
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


cli = click.CommandCollection(sources=[cli1, cli2, cli3])

if __name__ == '__main__':
    cli()
