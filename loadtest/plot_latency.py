import pandas as pd
import matplotlib.pyplot as plt
import sys
import os

def plot_latency(csv_file='latency.csv'):
    if not os.path.exists(csv_file):
        print(f"Error: File '{csv_file}' not found. Run the load test first.")
        return

    try:
        # Read the CSV data
        df = pd.read_csv(csv_file)
        
        # Create the plot
        plt.figure(figsize=(10, 6))
        
        # Plot Latency on primary y-axis
        fig, ax1 = plt.subplots(figsize=(12, 6))
        
        color = 'tab:red'
        ax1.set_xlabel('Time (s)')
        ax1.set_ylabel('Latency (ms)', color=color)
        l1, = ax1.plot(df['time'], df['avg_latency_ms'], color=color, label='Latency (ms)')
        ax1.tick_params(axis='y', labelcolor=color)
        ax1.grid(True, linestyle='--', alpha=0.7)

        # Create secondary y-axis for Active Users
        ax2 = ax1.twinx()  
        color = 'tab:blue'
        ax2.set_ylabel('Active Users', color=color)
        l2, = ax2.plot(df['time'], df['active_users'], color=color, linestyle='--', alpha=0.5, label='Active Users')
        ax2.tick_params(axis='y', labelcolor=color)

        # Title and Labels
        plt.title('MizuFlow Load Test: Latency & Concurrency over Time')
        
        # Legend
        lines = [l1, l2]
        labels = [l.get_label() for l in lines]
        ax1.legend(lines, labels, loc='upper left')

        # Save plot
        output_file = 'latency_plot.png'
        plt.savefig(output_file)
        print(f"✅ Plot saved to {output_file}")
        
        # Show plot (optional, might not work in headless env)
        # plt.show()

    except Exception as e:
        print(f"Error plotting data: {e}")

if __name__ == "__main__":
    csv_path = 'latency.csv'
    if len(sys.argv) > 1:
        csv_path = sys.argv[1]
    plot_latency(csv_path)
