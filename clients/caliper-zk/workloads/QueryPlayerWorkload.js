'use strict';

const { WorkloadModuleBase } = require('@hyperledger/caliper-core');

class QueryPlayerWorkload extends WorkloadModuleBase {
    constructor() {
        super();
    }

    async submitTransaction() {
        let playerID = 1000 + Math.floor(Math.random() * 1000); // Random player from 1000–1999

        let args = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'GetPlayer',
            contractArguments: [playerID.toString()],
            timeout: 60
        };

        await this.sutAdapter.sendRequests(args);
    }
}

function createWorkloadModule() {
    return new QueryPlayerWorkload();
}

module.exports.createWorkloadModule = createWorkloadModule;