import csv

import click
from influxdb import InfluxDBClient


@click.command()
@click.argument("run")
@click.option('-o', '--output',
              help="Output file to store the data to.",
              default=None, type=str)
@click.option('-t', '--ticks',
              help="Ampunt of ticks to process.",
              default=100, type=int)
def preprocess_run(run, output, ticks):
    # Initialize influx client
    client = InfluxDBClient(host="testground-influxdb", port=8086)
    client.switch_database("tetsground")

    # Create csv writer
    if not output:
        output = f"{run}.csv"
    f = open(output, "w")
    writer = csv.writer(f)

    peers = {}
    for point in client.query(
            'SHOW TAG VALUES from "diagnostics.casm-pex-convergence.view.point" WITH key=peer').get_points():
        peers[point["peer"]] = 0
    for tick in range(ticks):
        histogram_data = peers.copy()
        results = client.query(f'''SELECT * FROM "diagnostics.casm-pex-convergence.view.point" WHERE run='{run}' ''')
        for point in results.get_points(tags={"tick": tick}):
            for record in point["records"].split("-")[1:]:
                peers[record] += 1
        for key, value in histogram_data.values():
            writer.writerow([key, value, tick])
    f.close()


if __name__ == "__main__":
    preprocess_run()
