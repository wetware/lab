import re
import shlex
import subprocess
import time

import click


@click.command()
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
              type=str)
def emulate(min_node, max_node, step, view_size,
             convergence_threshold, repetitions, ticks, folder):
    command = f'testground daemon'
    with subprocess.Popen(shlex.split(command), text=True, stdout=subprocess.PIPE) as testground:
        time.sleep(3)  # TODO: read process output to know when is ready
        convergence_procs = []
        for nodes in range(min_node, max_node + 1, step):
            for rep in range(repetitions):
                command = f'testground run single --plan=casm --testcase="pex-convergence" ' \
                          f'--runner=local:docker --builder=docker:go --instances={nodes} ' \
                          f'--tp convTickAmount={ticks}'
                print(f"Running convergence with {nodes}/{max_node} nodes repetition {rep + 1}/{repetitions}...")
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
                        print(line)
                    line = testground.stdout.readline()

                print(f"Run convergence with {nodes}/{max_node} nodes repetition {rep + 1}/{repetitions}.")
                command = f"python3 convergence.py preprocess {run_id} -t {ticks}"
                if folder:
                    command += f" -f {folder}"
                proc = subprocess.Popen(shlex.split(command), stdout=subprocess.DEVNULL)
                convergence_procs.append((run_id, proc))

        print(f"Calculating convergence ticks...")
        for run_id, proc in convergence_procs:
            proc.wait()
            command = f"python3 convergence.py convergence-tick {run_id} " \
                      f"-vs {view_size} -t {ticks} " \
                      f"-o {convergence_procs[0][0]}"
            if folder:
                command += f" -f {folder}"
            for c in convergence_threshold:
                command += f" -c {c}"
            subprocess.run(shlex.split(command))
        print(f"Finished convergence tick calculation. "
              f"Saved at {convergence_procs[0][0]}.conv")
        testground.kill()



