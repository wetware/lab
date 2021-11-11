import os
import random
import re
import shlex
import string
import subprocess
import time

import click


@click.command()
@click.argument("min_node", type=int)
@click.argument("max_node", type=int)
@click.option("-s", "--step", type=int, default=1)
@click.option("-vs", "--view-size", type=int, default=32)
@click.option('-r', '--repetitions',
              help="Amount of times the convergence simulations is performed for every node amount.",
              default=1, type=int)
@click.option('-t', '--ticks',
              help="Amount of ticks to process.",
              default=100, type=int)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              type=str)
@click.option('--plot', help="Plot simulation convergence graph.",
              is_flag=True)
def emulate(min_node, max_node, step, view_size,
             repetitions, ticks, folder, plot):
    command = f'testground daemon'
    emulation_id = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    output_file = os.path.join(folder, f"{emulation_id}-pex.em") if folder else f"{emulation_id}-pex.em"
    with open(output_file, "w") as file:
        file.write(f"{min_node} {max_node} {repetitions} {step}\n")
    with subprocess.Popen(shlex.split(command), text=True, stdout=subprocess.PIPE) as testground:
        time.sleep(3)  # TODO: read process output to know when is ready
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
                with open(output_file, "a") as file:
                    file.write(f"{os.path.join(folder, run_id)}\n")

        print(f"Results stored at {output_file}")
        testground.kill()
    if plot:
        subprocess.run(shlex.split(f"python3 convergence.py plot {output_file}"))


if __name__ == "__main__":
    emulate()
