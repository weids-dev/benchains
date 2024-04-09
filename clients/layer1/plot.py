import json
import sys
import os
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

# Define the phases and rates used for benchmarking
phases = ['bexchange', 'exchange', 'bank']
rates = {'bexchange': [40, 60, 80, 100, 120, 140, 160, 180, 200, 220, 240], 'exchange': [40, 80, 120, 160, 240, 320, 360, 400, 440, 480], 'bank': [40, 80, 120, 160, 240, 320, 360, 400, 440, 480]}

# Directory containing the results
results_dir = 'results'

def process_results(results_dir):  # Add results_dir as a parameter
    for phase in phases:
        phase_data = []
        for rate in rates[phase]:
            filename = f"{results_dir}/{phase}_{rate}rps.json"
            if os.path.exists(filename):
                with open(filename, 'r') as f:
                    data = json.load(f)
                    phase_data.append({
                        'Rate': rate,
                        'Throughput': data['throughput'],
                        'Mean Latency (ms)': data['latencies']['mean'] / 1e6  # Convert from nanoseconds to milliseconds
                    })
            else:
                print(f"File not found: {filename}")
                
        # Convert to DataFrame and save to CSV
        df = pd.DataFrame(phase_data)
        csv_filename = f"{results_dir}/{phase}_results.csv"  # Adjust the file path
        df.to_csv(csv_filename, index=False)
        print(f"Saved {csv_filename}")
        
        # Generate chart
        plot_results(df, phase, results_dir)  # Pass results_dir to the plotting function

def detect_and_exclude_outliers(column):
    Q1 = column.quantile(0.25)
    Q3 = column.quantile(0.75)
    IQR = Q3 - Q1
    lower_bound = Q1 - 1.5 * IQR
    upper_bound = Q3 + 1.5 * IQR
    return column.between(lower_bound, upper_bound), lower_bound, upper_bound

def plot_results(df, phase, results_dir):
    fig, ax1 = plt.subplots(figsize=(10, 5))

    # Detect and exclude latency outliers
    is_not_outlier, _, _ = detect_and_exclude_outliers(df['Mean Latency (ms)'])
    df_filtered = df[is_not_outlier].reset_index(drop=True)
    rates_str = [str(rate) for rate in df_filtered['Rate']]
    x_indexes = np.arange(len(rates_str))

    color = 'tab:red'
    width = 0.35
    latency_bars = ax1.bar(x_indexes, df_filtered['Mean Latency (ms)'], color=color, width=width)
    ax1.set_xlabel('Transaction Arrival Rate (TPS)')
    ax1.set_ylabel('Latency (ms)', color=color)
    ax1.tick_params(axis='y', labelcolor=color)
    ax1.set_xticks(x_indexes)
    ax1.set_xticklabels(rates_str, rotation=45)

    for i, bar in enumerate(latency_bars):
        yval = bar.get_height()
        ax1.text(i, yval, round(yval, 1), ha='center', va='bottom', color='black', fontsize=8)

    # Adjusting Y-axis scale dynamically with some margins
    min_latency = df_filtered['Mean Latency (ms)'].min()
    max_latency = df_filtered['Mean Latency (ms)'].max()
    margin = (max_latency - min_latency) * 0.1 # 10% margin
    ax1.set_ylim([min_latency - margin, max_latency + margin])

        # Annotate outliers at the top-right corner
    for index, row in enumerate(df[~is_not_outlier].itertuples()):
        # Corrected to directly use attribute names based on DataFrame columns
        annotation_text = f"Outlier: {row.Rate} TPS, {round(row._3, 1)} ms"  # Adjust '_3' if 'Mean Latency (ms)' is at a different position
        ax1.annotate(annotation_text, (1, 1 - 0.05 * index), xycoords='axes fraction', xytext=(-5, -5), textcoords='offset points', ha='left', va='top', fontsize=8, color='red')

    ax2 = ax1.twinx()
    color = 'tab:blue'
    throughput_line = ax2.plot(x_indexes, df_filtered['Throughput'], color=color, marker='o', linewidth=2, label='Throughput (tps)')
    ax2.set_ylabel('Throughput (tps)', color=color)
    ax2.tick_params(axis='y', labelcolor=color)
    ax2.grid(False)

    for i, txt in enumerate(df_filtered['Throughput']):
        ax2.annotate(round(txt, 1), (x_indexes[i], txt), textcoords="offset points", xytext=(0,10), ha='center', fontsize=8)

    ax2.legend(loc='upper left')
    plt.title(f"{phase.capitalize()} Benchmark Results for {results_dir}")
    fig.tight_layout()
    
    chart_filename = f"{results_dir}/{phase}_chart.png"
    plt.savefig(chart_filename)
    plt.close(fig)
    print(f"Saved {chart_filename}")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python plot.py <results_directory>")
        sys.exit(1)

    results_dir = sys.argv[1]
    process_results(results_dir)
