'use strict';

const { WorkloadModuleBase } = require('@hyperledger/caliper-core');

class CreatePlayerWorkload extends WorkloadModuleBase {
    constructor() {
        super();
        this.txIndex = 0;
    }

    async initializeWorkloadModule(workerIndex, totalWorkers, roundIndex, roundArguments, sutAdapter, sutContext) {
        await super.initializeWorkloadModule(workerIndex, totalWorkers, roundIndex, roundArguments, sutAdapter, sutContext);
        this.totalWorkers = totalWorkers;
    }

    async submitTransaction() {
        // Generate unique ID starting from 1000 up to 1999
        // For 5 workers, txNumber: 1000 means 200 txs per worker
        // Worker 0: 1000, 1005, ..., 1995; Worker 1: 1001, 1006, ..., 1996; up to Worker 4: 1004, 1009, ..., 1999
        let id = 1000 + this.workerIndex + this.txIndex * this.totalWorkers;
        this.txIndex++;

        let args = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'CreatePlayer',
            contractArguments: [id.toString()],
            timeout: 60
        };

        await this.sutAdapter.sendRequests(args);
    }
}

function createWorkloadModule() {
    return new CreatePlayerWorkload();
}

module.exports.createWorkloadModule = createWorkloadModule;