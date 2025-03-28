'use strict';

const { WorkloadModuleBase } = require('@hyperledger/caliper-core');

class ExchangeCurrencyWorkload extends WorkloadModuleBase {
    constructor() {
        super();
    }

    async submitTransaction() {
        let playerID = 1000 + Math.floor(Math.random() * 1000); // Random player from 1000–1999
        let benAmountChange = 100; // Exchange 100 USD to 100 BEN

        let args = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'ExchangeInGameCurrency',
            contractArguments: [playerID.toString(), benAmountChange.toString()],
            timeout: 60
        };

        await this.sutAdapter.sendRequests(args);
    }
}

function createWorkloadModule() {
    return new ExchangeCurrencyWorkload();
}

module.exports.createWorkloadModule = createWorkloadModule;