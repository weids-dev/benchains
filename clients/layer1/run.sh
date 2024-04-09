#!/bin/bash

# Check for the correct number of arguments
if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <# endorsement nodes> <BatchTimeout> <MaxMessageCount>"
    exit 1
fi

./create.sh 480

# Assign the parameters to variables
endorsement_nodes=$1
batch_timeout=$2
max_message_count=$3

# Create a results directory based on the parameters
results_dir="results/${endorsement_nodes}_${batch_timeout}_${max_message_count}"
mkdir -p "${results_dir}"

declare -a deposit_exchange_rates=(40 80 120 160 200 240 280 320 360 400 440 480)
declare -a bexchange_rates=(40 60 80 100 120 140 160 180 200 220 240)

# Function to run Vegeta attack and save the report in JSON format
run_attack() {
    phase=$1
    rate=$2
    duration=$3
    outfile="${results_dir}/${phase}_${rate}rps.json"
    echo "Running ${phase} with rate ${rate} RPS"
    vegeta attack -workers 8 -targets="${phase}.txt" -rate=${rate} -duration=${duration}s | vegeta report -type=json > ${outfile}
    sleep 10
}

# Phase 1: Players (only once, adjust as needed)
run_attack "player" 240 "120"

# Phase 2 Deposit
for rate in "${deposit_exchange_rates[@]}"; do
    run_attack "bank" $rate "60"
done

# Phase 3 Exchange
for rate in "${deposit_exchange_rates[@]}"; do
    run_attack "exchange" $rate "60"
done

# Phase 4: Bexchange
for rate in "${bexchange_rates[@]}"; do
    run_attack "bexchange" $rate "60"
done

# Now call the Python script to process the results and generate the plots
# Make sure to pass the results directory as an argument to the script
python3 plot.py "${results_dir}"
