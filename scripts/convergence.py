import csv
import re
import shlex
import subprocess
import time
from os import path

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


@click.group()
def cli5():
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
@click.option('-i', '--interval',
              help="Speed at which ticks are plotted.", default=500)
@click.option('-p', '--is-preprocess',
              help="Flag to indicate you also want to pre-process.",
              is_flag=True)
@click.option('-pc', '--plot-convergence',
              help="Flag to indicate to plot convergence where "
                   "X is nodes amount and Y tick at which converges.",
              is_flag=True)
def plot(run, ticks, folder, interval, is_preprocess, plot_convergence):
    if plot_convergence:
        plot_line(run, ticks, folder, interval, is_preprocess)
    else:
        plot_histogram(run, ticks, folder, interval, is_preprocess)


def plot_line(run, ticks, folder, interval, is_preprocess):
    df = pd.read_csv(f"{path.join(folder, f'{run}.conv')}")
    thresholds = df["threshold"].unique()
    for t in thresholds:
        ax = df[df["threshold"] == t].groupby("nodes", as_index=False).mean().plot(x="nodes", y="tick")
        ax.yaxis.set_major_locator(MaxNLocator(integer=True))
        ax.xaxis.set_major_locator(MaxNLocator(integer=True))
        plt.title(f"Convergence threshold: {t}")
        plt.show()


def plot_histogram(run, ticks, folder, interval, is_preprocess):
    if is_preprocess:
        preprocess_run(run, ticks, folder)
    df = pd.read_csv(f"{path.join(folder, f'{run}.csv')}")
    instances = df["peerNum"].nunique()
    data = df.loc[df["tick"] == 1]["references"].values / instances

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
    ax.set_ylim(top=1)  # set safe limit to ensure that all data is visible.
    for count, rect in zip(data, bar_container.patches):
        rect.set_height(count)
    title = ax.text(0.5, 0.85, "", bbox={'facecolor': 'w', 'alpha': 0.5, 'pad': 5},
                    transform=ax.transAxes, ha="center")

    an = animation.FuncAnimation(fig, prepare_animation(bar_container), ticks,
                                 repeat=True, blit=False, interval=interval)
    plt.show()


@cli3.command()
@click.argument("run")
@click.option("-vs", "--view-size", type=int, default=32)
@click.option('-c', '--convergence-threshold',
              help="Convergence threshold.",
              default=[0.95], type=float, multiple=True)
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              default="out", type=str)
@click.option('-p', '--is-preprocess',
              help="Flag to indicate you also want to pre-process.",
              is_flag=True)
@click.option('-o', '--output',
              help="Output file to store the result to.",
              type=str)
def convergence_tick(run, view_size, convergence_threshold,
                     ticks, folder, is_preprocess, output):
    calculate_convergence_tick(run, view_size, convergence_threshold,
                               ticks, folder, is_preprocess, output)


def calculate_convergence_tick(run, view_size, convergence_threshold,
                               ticks, folder, is_preprocess, output):
    if is_preprocess:
        preprocess_run(run, ticks, folder)
    df = pd.read_csv(f"{path.join(folder, f'{run}.csv')}")
    instances = df["peerNum"].nunique()
    neighbors = min(instances - 1, view_size)

    output = output if output else run
    if folder:
        output = f"{folder}/{output}"
    output = f"{output}.conv"
    f = open(output, "a+")
    writer = csv.writer(f)
    with open(output, "r") as read_file:
        if not read_file.read():
            writer.writerow(["nodes", "threshold", "tick"])

    for c in convergence_threshold:
        for tick in range(1, ticks):
            data = df.loc[df["tick"] == tick]["references"].values
            if (neighbors * c <= data).all():
                writer.writerow([instances, c, tick])
                print(f"{c} convergence with {instances} nodes holds at tick {tick} with {instances} nodes")
                break
    f.close()


@cli4.command()
@click.argument("min_node", type=int)
@click.argument("max_node", type=int)
@click.option("-s", "--step", type=int, default=1)
@click.option("-vs", "--view-size", type=int, default=32)
@click.option('-c', '--convergence-threshold',
              help="Convergence threshold.",
              default=[0.95], type=float, multiple=True)
@click.option('-r', '--repetitions',
              help="Amount of times the convergence simulations is performed for every node amount.",
              default=1, type=int)
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              default="out", type=str)
@click.option('-p', '--is-preprocess',
              help="Flag to indicate you also want to pre-process.",
              is_flag=True)
def converge(min_node, max_node, step, view_size,
             convergence_threshold, repetitions, ticks, folder, is_preprocess):
    run_converge(min_node, max_node, step, view_size, convergence_threshold, repetitions, ticks, folder, is_preprocess)


def run_converge(min_node, max_node, step, view_size, convergence_threshold, repetitions, ticks, folder, is_preprocess):
    command = f'testground daemon'
    with subprocess.Popen(shlex.split(command), text=True, stdout=subprocess.PIPE) as testground:
        time.sleep(2)
        convergence_procs = []
        for nodes in range(min_node, max_node+1, step):
            for rep in range(repetitions):
                command = f'testground run single --plan=casm --testcase="pex-convergence" ' \
                      f'--runner=local:docker --builder=docker:go --instances={nodes} ' \
                      f'--tp convTickAmount={ticks}'
                print(f"Running convergence with {nodes}/{max_node} nodes repetition {rep+1}/{repetitions}...")
                proc = subprocess.run(shlex.split(command), text=True, stdout=subprocess.PIPE)
                if not re.search("run is queued with ID: (.+)\\n", proc.stdout):
                    print(proc.stdout)
                run_id = re.search("run is queued with ID: (.+)\\n", proc.stdout).group(1)
                line = testground.stdout.readline()
                while line:
                    if re.search("Tick [0-9]+/[0-9]+", line):
                        print(line)
                    if re.search(f"Tick {ticks}/", line):
                        break
                    if re.search("ERROR", line):
                        return
                    line = testground.stdout.readline()

                print(f"Run convergence with {nodes}/{max_node} nodes.")
                command = f"python3 convergence.py preprocess {run_id} -t {ticks}"
                proc = subprocess.Popen(shlex.split(command), text=True, stdout=subprocess.PIPE)
                convergence_procs.append((run_id, proc))

        print(f"Calculating convergence ticks...")
        for run_id, proc in convergence_procs:
            proc.wait()
            command = f"python3 convergence.py convergence-tick {run_id} " \
                      f"-vs {view_size} -t {ticks} " \
                      f"-o {convergence_procs[0][0]}"
            for c in convergence_threshold:
                command += f" -c {c}"
            subprocess.run(shlex.split(command))
        print(f"Finished convergence tick calculation. "
              f"Saved at {convergence_procs[0][0]}.csv")
        return


cli = click.CommandCollection(sources=[cli1, cli2, cli3, cli4])

if __name__ == '__main__':
    cli()
