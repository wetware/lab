import csv

import click
from influxdb import InfluxDBClient


@click.command()
@click.argument("run")
@click.option('-t', '--ticks',
              help="Ampunt of ticks to process.",
              default=100, type=int)
@click.option('-o', '--output',
              help="Output file to store the data to.",
              default=None, type=str)
@click.option('-f', '--folder',
              help="Output folder to store the file to.",
              default="out", type=str)
def preprocess_run(run, ticks, output, folder):
    # Initialize influx client
    client = InfluxDBClient(host="localhost", port=8086)
    client.switch_database("testground")

    # Create csv writer
    if not output:
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
        for point in results.get_points(tags={"tick": str(tick)}):
            if not point["records"]:
                continue
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


if __name__ == "__main__":
    preprocess_run()
