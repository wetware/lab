import click
import matplotlib.animation as animation
import matplotlib.pyplot as plt
import pandas as pd


@click.command()
@click.argument("run")
@click.option('-t', '--ticks',
              help="Ampunt of ticks to process.",
              default=100, type=int)
def plot_histogram(run, ticks):
    df = pd.read_csv(run)
    instances = df["peerNum"].nunique()
    data = df.loc[df["tick"] == 1]["references"].values

    # histogram our data with numpy

    def prepare_animation(bar_container):
        def animate(frame_number):
            # simulate new data coming in
            plt.title(f"Tick {frame_number+1}")
            data = df.loc[df["tick"] == frame_number + 1]["references"].values
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
                                  repeat=False, blit=False)
    plt.show()


if __name__ == "__main__":
    plot_histogram()
