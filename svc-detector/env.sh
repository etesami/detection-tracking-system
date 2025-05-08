#!/bin/bash
# Generate environment variables for the service aggregator
# Primarily used for Histogram Buckets

# SENT_DATA_BYTE_BUCKETS
export SENT_DATA_BYTE_BUCKETS="$(
  {
    seq 100000 50000 1000000
    seq 1100000 100000 2000000
    seq 2500000 500000 5000000
  } | paste -sd,
)"

# TRANSMIT_TIME_BUCKETS, E2E_TIME_BUCKETS
export TRANSMIT_TIME_BUCKETS="$(
  {
    seq 0.5 0.5 20
    seq 21 1 40
    seq 42 2 100
    seq 105 5 200
    seq 210 10 600
    seq 620 20 1000
    seq 1050 50 1500
    seq 1600 100 5000
    seq 5500 500 10000
  } | paste -sd,
)"

export PROC_TIME_BUCKETS="$(
  {
    seq 0.005 0.005 0.01
    seq 0.05 0.05 0.1
    seq 0.2 0.2 4
    seq 4.5 0.5 10
    seq 11 1 20
    seq 22 2 40
    seq 45 5 90
    seq 100 10 200
    seq 250 50 400
    seq 500 100 1000
    seq 1500 500 5000
  } | paste -sd,
)"

echo "SENT_DATA_BYTE_BUCKETS=$SENT_DATA_BYTE_BUCKETS" > .env_histogram
echo "TRANSMIT_TIME_BUCKETS=$TRANSMIT_TIME_BUCKETS" >> .env_histogram
echo "E2E_TIME_BUCKETS=$TRANSMIT_TIME_BUCKETS" >> .env_histogram
echo "PROC_TIME_BUCKETS=$PROC_TIME_BUCKETS" >> .env_histogram