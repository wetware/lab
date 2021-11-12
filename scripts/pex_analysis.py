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
def plot(run: str, tick: List[int], dd: bool, pth: bool, cc: bool, mtx: bool, all:bool):
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

@cli1.command()
@click.argument("run", type=str)
@click.option("-t", "--tick", type=int, multiple=True, default=[1, 100])
def calculate(run:str, tick: List[int]):
    pass  # TODO


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
    return graph


cli = click.CommandCollection(sources=[cli1, cli2, cli3])

if __name__ == '__main__':
    cli()
