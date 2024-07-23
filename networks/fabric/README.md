# Fabric

## Overview
This repository contains the configuration code for multiple Hyperledger Fabric networks, meticulously designed to simulate a public blockchain environment akin to systems like Ethereum. As I mentioned in the [README](https://github.com/weids-dev/benchains) of this project, our intent with this project is to construct a simple yet effective network that forms the backbone of our benchmarking framework, utilizing the robust capabilities of Hyperledger Fabric. This approach closely adheres to the configuration files and standards outlined in the insightful official Hyperledger Fabric [documentation](https://hyperledger-fabric.readthedocs.io/en/latest/tutorials.html), and includes customized settings such as endorsement policies and organization configurations.

The rationale behind employing a private blockchain system like Hyperledger Fabric for constructing our Layer 1 blockchain is rooted in strategy. Such private systems afford us an enhanced level of control over all network nodes, significantly benefiting the management and monitoring aspects of our research. This control is essential, as it empowers us to maintain a streamlined design for the entire system while ensuring the accuracy and dependability of our experimental outcomes. 

The codebase is structured with simplicity and clarity in mind. In the `scripts/` directory, you will find straightforward shell scripts that handle the routine tasks of configuring the network using the settings found in the `config/` and `slim/` directory. These scripts leverage various Hyperledger Fabric binaries, as illustrated in the official tutorials.

## Prerequisites
Ensure that Docker is installed on your system and that the [Docker](https://docs.docker.com/engine/install/) daemon is running.

## Getting Started (Layer 1 Network)

```shell
Usage: ./fabric.sh [options]
Options:
  -i   Install the fabric binaries.
  -c   Create certificates for the network components.
  -n   Start the network nodes (peers and orderers).
  -t   Stop the network nodes.
  -l   Create the network channel and join nodes.
  -s   Install and set up the sample chaincode.
  -a   Invoke the installed chaincode.
  -h   Display this help message.
```

### 1. Install the Fabric Binaries
To install the required Hyperledger Fabric binaries, navigate to the `scripts/` directory and execute the `fabric.sh` script with the `-i` flag.

```bash
cd scripts && ./fabric.sh -i
```

### 2. Create Certificates
Generate the necessary certificates for the network components by running:
```bash
./fabric.sh -c
```

### 3. Bring the Network Up
Start the peers and orderers, the create the network and join nodes to the channel.
```bash
./fabric.sh -nl
```

### 4. Install Sample Chaincode for Testing
```bash
./fabric.sh -sa
```

If you see the following output, which means the chaincode is being endorsed in all peers of the network:

```
INFO [chaincodeCmd] chaincodeInvokeOrQuery -> Chaincode invoke successful. result: status:200
```

## Bring up a Layer 2 Network

```bash
./fabric.sh -dew
```

```
# A Peer Node Join Both Layer 1 & Layer 2 Networks
# Layer 2 Network Chaincode Invoke 
Committed chaincode definition for chaincode 'pasic' on channel 'chains02':
Version: 1.0, Sequence: 1, Endorsement Plugin: escc, Validation Plugin: vscc, Approvals: [org01MSP: true, org02MSP: true]
GetAllPlayers
[{"id":"AWANG","balance":1020.0000000000001,"items":[]},{"id":"player1","balance":0,"items":[]},{"id":"player2","balance":0,"items":[]},{"id":"player3","balance":0,"items":[]}]

# Layer 1 Network Chaincode Invoke
Committed chaincode definition for chaincode 'basic' on channel 'chains':
Version: 1.0, Sequence: 1, Endorsement Plugin: escc, Validation Plugin: vscc, Approvals: [org01MSP: true]
GetAllPlayers
[{"id":"AWANG","balance":960,"items":[]},{"id":"player1","balance":0,"items":[]},{"id":"player2","balance":0,"items":[]},{"id":"player3","balance":0,"items":[]}]
```

## Contributions and Feedback
As this project is in a phase of active development, contributions and feedback are highly welcomed. Please feel free to raise issues or submit pull requests as you see fit.
