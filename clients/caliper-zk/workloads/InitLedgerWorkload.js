'use strict';

const { WorkloadModuleBase } = require('@hyperledger/caliper-core');

class InitLedgerWorkload extends WorkloadModuleBase {
    constructor() {
        super();
    }

    async submitTransaction() {
        let args = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'InitLedger',
            contractArguments: [],
            timeout: 10
        };

        await this.sutAdapter.sendRequests(args);
    }
}

function createWorkloadModule() {
    return new InitLedgerWorkload();
}

module.exports.createWorkloadModule = createWorkloadModule;