import csv
import os.path
import random
import string

import click
import matplotlib.animation as animation
import matplotlib.pyplot as plt
import pandas as pd
from influxdb import InfluxDBClient
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


@click.group()
def cli4():
    pass


@cli2.command()
@click.argument("input_file")
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option("-vs", "--view-size", type=int, default=32)
@click.option('-ct', '--convergence-threshold',
              help="Convergence threshold.",
              default=[0.95, 0.99], type=float, multiple=True)
@click.option('-ca', '--cache',
              is_flag=True)
def plot(input_file, ticks, view_size, convergence_threshold, cache):
    convergence_file = converge(input_file, view_size, convergence_threshold, ticks, cache)
    plot_line(convergence_file, convergence_threshold)


def plot_line(input_file, convergence_threshold):
    df = pd.read_csv(input_file)
    for t in convergence_threshold:
        ax = df[df["threshold"] == t].groupby("nodes", as_index=False).mean().plot(x="nodes", y="tick")
        ax.yaxis.set_major_locator(MaxNLocator(integer=True))
        ax.xaxis.set_major_locator(MaxNLocator(integer=True))
        plt.title(f"Convergence threshold: {t}")
        plt.show()


@cli3.command()
@click.argument("input_file")
@click.option("-n", "--nodes", default=[3], type=int, multiple=True)
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option("-vs", "--view-size", type=int, default=32)
@click.option('-it', '--interval',
              help="Speed at which ticks are plotted.", default=500)
@click.option('-ca', '--cache',
              is_flag=True)
def plot_hist(input_file, nodes, ticks, view_size, interval, cache):
    with open(input_file, "r") as f:
        lines = f.read().splitlines()
        min_n, max_n, rep, step = map(lambda e: int(e), lines[0].split(" "))
        runs = lines[1:]

    for n in nodes:
        index = min(max((n - min_n) // step, 0) * rep, len(runs) - 1)
        dfs = []
        for run in runs[index:index + rep]:
            file = run + ".csv"
            if not os.path.isfile(file):
                folder, run = os.path.split(run)
                preprocess(run, ticks, folder if folder else None)
                if cache:
                    print(f"Preprocessed run {run} and storing at {file}")
            dfs.append(pd.read_csv(file))
            if not cache:
                os.remove(file)
        df = pd.concat(dfs)
        df = df.groupby(df.index).mean()
        df = df.astype({"nodeNum": "int64", "tick": "int64"})

        ticks = min(df["tick"].max(), ticks)
        instances = df["nodeNum"].nunique()
        max_value = min(instances - 1, view_size)
        data = df.loc[df["tick"] == 1]["references"].values / max_value  # Normalize data
        def prepare_animation(bar_container):
            def animate(frame_number):
                # simulate new data coming in
                plt.title(f"{min_n+((index//rep)*step)} nodes - Tick {frame_number + 1}")
                data = df.loc[df["tick"] == frame_number + 1]["references"].values / max_value  # Normalize data
                for count, rect in zip(data, bar_container.patches):
                    rect.set_height(count)
                return bar_container.patches

            return animate

        fig, ax = plt.subplots()
        _, _, bar_container = ax.hist(data, instances, lw=1,
                                      ec="yellow", fc="green", alpha=0.5)
        ax.set_ylim(top=1)  # set safe limit to ensure that all data is visible.
        for count, rect in zip(data, bar_container.patches):
            rect.set_height(count)
        title = ax.text(0.5, 0.85, "", bbox={'facecolor': 'w', 'alpha': 0.5, 'pad': 5},
                        transform=ax.transAxes, ha="center")

        an = animation.FuncAnimation(fig, prepare_animation(bar_container), ticks,
                                     repeat=True, blit=False, interval=interval)
        plt.show()


def converge(input_file, view_size, convergence_threshold,
             ticks, cache):
    with open(input_file, "r") as f:
        lines = f.read().splitlines()
        min_n, max_n, rep, step = map(lambda e: int(e), lines[0].split(" "))
        runs = lines[1:]

    output_file = ".".join(input_file.split(".")[:-1]) + ".conv"
    for i in range(0, len(runs), rep):
        dfs = []
        for run in runs[i:i + rep]:
            file = run + ".csv"
            if not os.path.isfile(file):
                folder, run = os.path.split(run)
                preprocess(run, ticks, folder if folder else None)
                if cache:
                    print(f"Preprocessed run {run} and storing at {file}")
            dfs.append(pd.read_csv(file))
            if not cache:
                os.remove(file)
        df = pd.concat(dfs)
        df = df.groupby(df.index).mean()
        df = df.astype({"nodeNum": "int64", "tick": "int64"})
        instances = df["nodeNum"].nunique()

        with open(output_file, "a") as f:
            with open(output_file, "r") as read_file:
                if not read_file.read():
                    writer = csv.writer(f)
                    writer.writerow(["nodes", "threshold", "tick"])

        max_value = min(instances - 1, view_size)
        for c in convergence_threshold:
            has_converged = False
            for tick in range(1, min(ticks, df["tick"].max())):
                data = df.loc[df["tick"] == tick]["references"].values / max_value
                if (c <= data).all():
                    with open(output_file, "a") as f:
                        writer = csv.writer(f)
                        writer.writerow([instances, c, tick])
                    print(f"{c} convergence with {instances} nodes holds at tick {tick} with {instances} nodes")
                    has_converged = True
                    break
            if not has_converged:
                with open(output_file, "a") as f:
                    writer = csv.writer(f)
                    writer.writerow([instances, c, 0])
                print(f"{c} does not converge after {min(ticks, df['tick'].nunique())} ticks with {instances} nodes")

    print("Convergence results stored at", output_file)
    return output_file


def preprocess(run, ticks, folder):
    # Initialize influx client
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")

    # Create csv writer
    output = f"{folder}/{run}.csv" if folder else f"{run}.csv"

    with open(output, "w") as f:
        writer = csv.writer(f)
        writer.writerow(["nodeNum", "references", "tick"])

    nodes = {}
    nodes_seq = {}
    for i, point in enumerate(client.query(
            f'''SHOW TAG VALUES from "diagnostics.casm-pex-convergence.view.point" WITH key=node WHERE run='{run}' ''').get_points()):
        nodes[i] = 0
        nodes_seq[point["value"]] = i
    for tick in range(1, ticks + 1):
        histogram_data = nodes.copy()
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
                    histogram_data[nodes_seq[record]] += 1
        for node, references in histogram_data.items():
            with open(output, "a") as f:
                writer = csv.writer(f)
                writer.writerow([node, references, tick])


cli = click.CommandCollection(sources=[cli1, cli2, cli3, cli4])

if __name__ == '__main__':
    cli()
