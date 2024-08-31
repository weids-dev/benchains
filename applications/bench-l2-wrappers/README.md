# Bench-l2-wrappers

This directory contains the gateway application and chaincode designed to simulate Layer 2 Plasma chains and Settlement Agents on a Hyperledger Fabric network, based on the [Minimal Viable Plasma](https://ethresear.ch/t/minimal-viable-plasma/426) (Plasma MVP) model by Vitalik Buterin.

## Overview

The gateway application acts as a Settlement Agent that:
1. Fetches the newest blocks and write sets from transactions on the Plasma chain.
2. Communicates with peers from both the root chain (`org01 chains`) and the Plasma chain (`org02 chains02`).
3. Periodically checks the latest blocks, computes the Merkle tree root, and commits it to the root chain.

## Getting Started
To start the application:
```shell
go mod tidy
go run main.go
```
