# Benchains

Benchmarking various Layer 2 blockchains solutions using Hyperledger Fabric.

## Background
As part of my research at **[HKBU](https://www.comp.hkbu.edu.hk/)** in [MSc Research I](https://www.comp.hkbu.edu.hk/v1/file/course/COMP7960.pdf), I am conducting a comprehensive survey on blockchain technologies, with a focus on Layer 2 solutions.

Although existing surveys provided a foundation, I identified a gap in the benchmarking of different blockchain layer 2 solutions.
Notably, existing benchmarks like [Fabric](https://ieeexplore.ieee.org/document/8526892), [Blockbench](https://www.comp.nus.edu.sg/~ooibc/blockbench.pdf) and [BBSF](https://dl.acm.org/doi/10.1145/3595647.3595649) primarily focus on:
1. Evaluating the performance of different blockchain systems under identical conditions.
2. Impact of Layer 1 attributes (block size, consensus algorithms, node count) on single blockchain system performance.

However, there's a lack of benchmarking for different Layer 2 solutions.
This gap is acknowledged in the recent [BBSF](https://dl.acm.org/doi/pdf/10.1145/3595647.3595649) paper, which underscores the necessity for specialized workloads to better understand Layer 2 performance dynamics.

This **benchains** project is designed to address this gap. The primary challenge lies in the diverse use cases and varying efficacy of Layer 2 solutions across different blockchains. For example, the [Lightning Network](https://lightning.network/lightning-network-paper.pdf) is optimized for high-frequency transactions in bitcoin, meanwhile scaling solutions like [Plasma](https://ethereum.org/en/developers/docs/scaling/plasma/) and [ZK-rollups](https://ethereum.org/en/developers/docs/scaling/zk-rollups/) are designed to increase throughput my moving computation & state-stroage off Ethereum's mainchain.

## Methodology
To address this, I propose a method that is straightforward enough with a simple design:

1. **Creating a Simulated Layer 1 Blockchain Using Fabric**:
We'll construct a simple layer 1 network that emulates a public blockchain system (such as Ethereum) using [Hyperledger Fabric](https://www.hyperledger.org/projects/fabric). This network will form the foundation of our benchmarking framework. 

The choice to use a private blockchain system like Hyperledger Fabric for building our Layer 1 blockchain is strategic. Private blockchain systems offer greater control over all nodes in the system, allowing for enhanced management and monitoring capabilities. 
This control is pivotal for our research, as it enables us to maintain a straightforward design for the entire system while ensuring the precision and reliability of our experiments. The use of a private blockchain like Hyperledger Fabric allows us to create a more predictable and manageable benchmarking environment without modifying the source code of the underlying Fabric framework. This aspect is crucial for accurately assessing the performance of Layer 2 solutions. By not altering the core Fabric code, we ensure that our findings are relevant and applicable to standard deployments, enhancing the validity and utility of our research.

At the same time, the **chaincode** and **channel** features provided by Hyperledger Fabric offer rich controls for manipulating states both in the Layer 1 environment and throughout all subsequent steps in building and benchmarking different Layer 2 solutions. These features enable a high degree of customization and flexibility, allowing us to tailor the network to suit our specific benchmarking needs while maintaining a consistent and controlled experimental environment.

2. **Developing Workloads for Full Load Simulation**:
The next step involves creating client application workloads to fully utilize each peer's computational power, ensuring the throughput is CPU-bound.
This could involve simulating a database and contrasting it with a distributed system using a relational database.

3. **Integrating and Benchmarking Layer 2 Solutions**:
The final and most challenging step is integrating various Layer 2 solutions into our network.
Our focus will be on assessing the benefits and optimal use cases of each solution.
For instance, leveraging Fabric's channel mechanisms can facilitate implementing Lightning Network channels and sidechains, allowing for network behavior modification through chaincode.

## Implementation
### 1. Creating a Simulated Layer 1 Blockchain Using Fabric
The foundational simulated Layer 1 network using Fabric has been developed in [Angold-4/chains](https://github.com/Angold-4/chains) as of December 2023. All relevant code and documentation have been migrated to [`networks/fabric/`](https://github.com/weids-dev/benchains/tree/main/networks/fabric) in this repository.

