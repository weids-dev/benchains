import matplotlib.pyplot as plt
import numpy as np
import pandas as pd
import sys
import os

def load_and_plot_data(file_paths, base_name, num_rates):
    fig, ax1 = plt.subplots(figsize=(10, 5))
    color_palette = plt.cm.viridis(np.linspace(0, 1, len(file_paths)))
    marker_list = ['o', 'v', '^', '<', '>', 's', 'p', '*', 'h', 'H', 'D', 'd']

    for i, file_path in enumerate(file_paths):
        df = pd.read_csv(file_path).head(num_rates)
        rates_str = [str(rate) for rate in df['Rate']]
        x_indexes = np.arange(len(rates_str))

        # Plot latency
        latency_bars = ax1.bar(x_indexes + i * 0.1, df['Mean Latency (ms)'], color=color_palette[i], width=0.1, label=f'{os.path.basename(os.path.dirname(file_path))}')

        # Annotate each bar with its value
        for bar in latency_bars:
            height = bar.get_height()
            ax1.annotate(f'{round(height, 1)}',
                         xy=(bar.get_x() + bar.get_width() / 2, height),
                         xytext=(0, 3),  # 3 points vertical offset
                         textcoords="offset points",
                         ha='center', va='bottom')



        # Plot throughput with a secondary axis
        if i == 0:  # Only need to create the secondary axis once
            ax2 = ax1.twinx()

        ax2.plot(x_indexes, df['Throughput'], color=color_palette[i], marker=marker_list[i % len(marker_list)], linewidth=2, label=f'Throughput: {os.path.basename(os.path.dirname(file_path))}')

    ax1.set_xlabel('Transaction Arrival Rate (TPS)')
    ax1.set_ylabel('Latency (ms)', color='tab:red')
    ax2.set_ylabel('Throughput (tps)', color='tab:blue')
    ax1.tick_params(axis='y', labelcolor='tab:red')
    ax2.tick_params(axis='y', labelcolor='tab:blue')
    ax1.set_xticks(x_indexes)
    ax1.set_xticklabels(rates_str, rotation=45)
    ax1.legend(loc='upper left', fontsize='small', title="Latency", title_fontsize='medium')  # Add title and adjust title font size here
    plt.title(f"Combined Benchmark Results")
    fig.tight_layout()

    chart_filename = f"results/{base_name}_combined_chart.png"
    plt.savefig(chart_filename)
    plt.close(fig)
    print(f"Saved {chart_filename}")

if __name__ == "__main__":
    if len(sys.argv) < 4:
        print("Usage: python combine.py <base_name> <num_rates> <dir_1> <dir_2> ... <dir_n>")
    else:
        base_name = sys.argv[1]
        num_rates = int(sys.argv[2])  # New parameter to specify the number of rates
        dirs = sys.argv[3:]
        file_paths = [f"results/{dir}/{base_name}_results.csv" for dir in dirs]
        load_and_plot_data(file_paths, base_name, num_rates)

