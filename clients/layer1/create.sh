#!/bin/bash

# Usage: ./create.sh RATE
# RATE is the number of requests per second you plan to test with.

RATE=$1
if [ -z "$RATE" ]; then
  echo "Please provide a rate."
  exit 1
fi

# Calculate the number of requests for each phase based on the rate and duration (60 seconds per phase)
REQUESTS_PER_PHASE=$(($RATE * 60))

# Phase 1: Create players
{
  for ((i=1; i<=REQUESTS_PER_PHASE; i++)); do
    echo "PUT http://192.168.50.29:10808/player/user${i}"
    echo ""
  done
} > player.txt

# Phase 2: Deposit transactions for each player
{
  for ((i=1; i<=REQUESTS_PER_PHASE; i++)); do
    echo "PUT http://192.168.50.29:10808/bank/txID${i}/3/user${i}"
    echo ""
  done
} > bank.txt

# Phase 3: Exchange for each player
{
  for ((i=1; i<=REQUESTS_PER_PHASE; i++)); do
    echo "PUT http://192.168.50.29:10808/exchange/txID${i}/user${i}"
    echo ""
  done
} > exchange.txt

# Phase 4: Combined Deposit-Exchange for each player (using a different set of txIDs)
{
  for ((i=1; i<=REQUESTS_PER_PHASE; i++)); do
    # Assuming txID starts from REQUESTS_PER_PHASE + 1 to ensure uniqueness
    offset=$((REQUESTS_PER_PHASE + i))
    echo "PUT http://192.168.50.29:10808/bexchange/txID${offset}/3/user${i}"
    echo ""
  done
} > bexchange.txt

echo "Generated files for three phases with a total of $(($REQUESTS_PER_PHASE * 4)) requests."
