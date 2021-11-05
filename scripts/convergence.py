import csv
from os import path

import click
import matplotlib.animation as animation
import matplotlib.pyplot as plt
import pandas as pd
from influxdb import InfluxDBClient


@click.group()
def cli1():
    pass


@click.group()
def cli2():
    pass


@cli1.command()
@click.argument("run")
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              default="out", type=str)
def preprocess(run, ticks, folder):
    preprocess_run(run, ticks, folder)


def preprocess_run(run, ticks, folder):
    # Initialize influx client
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")

    # Create csv writer
    output = f"{run}.csv"
    if folder:
        output = f"{folder}/{output}"
    f = open(output, "w")
    writer = csv.writer(f)

    peers = {}
    peers_seq = {}
    for i, point in enumerate(client.query(
            f'''SHOW TAG VALUES from "diagnostics.casm-pex-convergence.view.point" WITH key=peer WHERE run='{run}' ''').get_points()):
        peers[i] = 0
        peers_seq[point["value"]] = i
    writer.writerow(["peerNum", "references", "tick"])
    for tick in range(1, ticks + 1):
        histogram_data = peers.copy()
        # Extract peer views at tick 'tick'
        results = client.query(f'''SELECT * FROM "diagnostics.casm-pex-convergence.view.point" WHERE run='{run}' ''')
        found = set()
        for point in reversed(list(results.get_points(tags={"tick": str(tick)}))):
            if not point["records"]:
                continue
            if point["peer"] in found:
                continue
            else:
                found.add(point["peer"])

            for record in point["records"].split("-")[1:]:
                histogram_data[peers_seq[record]] += 1
        for key, value in histogram_data.items():
            writer.writerow([key, value, tick])
    f.close()

    # Write peer->id mapping
    output = f"{''.join(output.split('.')[:-1])}.peers"
    f = open(output, "w")
    writer = csv.writer(f)
    writer.writerow(["peerID", "peerNum"])
    for peer, i in peers_seq.items():
        writer.writerow([i, peer])
    f.close()


@cli2.command()
@click.argument("run")
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              default="out", type=str)
@click.option('-p', '--is-preprocess',
              help="Flag to indicate you also want to pre-process.",
              is_flag=True)
def plot(run, ticks, folder, is_preprocess):
    plot_histogram(run, folder, ticks, is_preprocess)


def plot_histogram(run, folder, ticks, is_preprocess):
    if is_preprocess:
        preprocess_run(run, ticks, folder)
    df = pd.read_csv(f"{path.join(folder, f'{run}.csv')}")
    instances = df["peerNum"].nunique()
    data = df.loc[df["tick"] == 1]["references"].values

    # histogram our data with numpy

    def prepare_animation(bar_container):
        def animate(frame_number):
            # simulate new data coming in
            plt.title(f"Tick {frame_number + 1}")
            data = df.loc[df["tick"] == frame_number + 1]["references"].values / instances
            for count, rect in zip(data, bar_container.patches):
                rect.set_height(count)
            return bar_container.patches

        return animate

    fig, ax = plt.subplots()
    _, _, bar_container = ax.hist(data, instances, lw=1,
                                  ec="yellow", fc="green", alpha=0.5)
    ax.set_ylim(top=instances)  # set safe limit to ensure that all data is visible.
    title = ax.text(0.5, 0.85, "", bbox={'facecolor': 'w', 'alpha': 0.5, 'pad': 5},
                    transform=ax.transAxes, ha="center")

    an = animation.FuncAnimation(fig, prepare_animation(bar_container), ticks,
                                 repeat=True, blit=False)
    plt.show()


@cli2.command()
@click.argument("run")
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              default="out", type=str)
@click.option('-p', '--is-preprocess',
              help="Flag to indicate you also want to pre-process.",
              is_flag=True)
def convergence_tick(run, ticks, folder, is_preprocess):
    convergence_tick(run, folder, ticks, is_preprocess)


def convergence_tick(run, folder, ticks, is_preprocess):
    if is_preprocess:
        preprocess_run(run, ticks, folder)
    df = pd.read_csv(f"{path.join(folder, f'{run}.csv')}")
    instances = df["peerNum"].nunique()
    data = df.loc[df["tick"] == 1]["references"].values

    # TODO



cli = click.CommandCollection(sources=[cli1, cli2])

if __name__ == '__main__':
    cli()
