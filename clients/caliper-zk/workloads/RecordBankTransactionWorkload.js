'use strict';

const { WorkloadModuleBase } = require('@hyperledger/caliper-core');

class RecordBankTransactionWorkload extends WorkloadModuleBase {
    constructor() {
        super();
        this.txIndex = 0;
    }

    async initializeWorkloadModule(workerIndex, totalWorkers, roundIndex, roundArguments, sutAdapter, sutContext) {
        await super.initializeWorkloadModule(workerIndex, totalWorkers, roundIndex, roundArguments, sutAdapter, sutContext);
        this.workerIndex = workerIndex;
        this.totalWorkers = totalWorkers;
        this.startPlayer = 1000 + this.workerIndex * 200; // Assign 200 players per worker
    }

    async submitTransaction() {
        let playerID = this.startPlayer + (this.txIndex % 200); // Cycle through 200 players
        let transactionID = this.workerIndex * 1000000 + this.txIndex; // Unique transaction ID
        this.txIndex++;
        let amountUSD = 10000; // Deposit 10,000 USD

        let args = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'RecordBankTransaction',
            contractArguments: [playerID.toString(), amountUSD.toString(), transactionID.toString()],
            timeout: 60
        };

        await this.sutAdapter.sendRequests(args);
    }
}

function createWorkloadModule() {
    return new RecordBankTransactionWorkload();
}

module.exports.createWorkloadModule = createWorkloadModule;