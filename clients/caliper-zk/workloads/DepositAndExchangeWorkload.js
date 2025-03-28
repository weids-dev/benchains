'use strict';

const { WorkloadModuleBase } = require('@hyperledger/caliper-core');

class DepositAndExchangeWorkload extends WorkloadModuleBase {
    constructor() {
        super();
        this.txIndex = 0;
    }

    async initializeWorkloadModule(workerIndex, totalWorkers, roundIndex, roundArguments, sutAdapter, sutContext) {
        await super.initializeWorkloadModule(workerIndex, totalWorkers, roundIndex, roundArguments, sutAdapter, sutContext);
        this.workerIndex = workerIndex;
    }

    async submitTransaction() {
        let playerID = 1000 + Math.floor(Math.random() * 1000); // Random player from 1000–1999
        let transactionID = this.workerIndex * 1000000 + this.txIndex; // Unique transaction ID
        this.txIndex++;
        let amountUSD = 100; // Deposit 100 USD
        let benAmountChange = 100; // Exchange 100 USD to 100 BEN

        let args1 = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'RecordBankTransaction',
            contractArguments: [playerID.toString(), amountUSD.toString(), transactionID.toString()],
            timeout: 60
        };

        let args2 = {
            contractId: 'pasic',
            contractVersion: 'v1',
            contractFunction: 'ExchangeInGameCurrency',
            contractArguments: [playerID.toString(), benAmountChange.toString()],
            timeout: 60
        };

        await this.sutAdapter.sendRequests(args1); // Deposit first
        await this.sutAdapter.sendRequests(args2); // Then exchange
    }
}

function createWorkloadModule() {
    return new DepositAndExchangeWorkload();
}

module.exports.createWorkloadModule = createWorkloadModule;