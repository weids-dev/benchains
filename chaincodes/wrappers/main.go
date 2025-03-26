package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	"github.com/weids-dev/benchains/chaincodes/wrappers/currency"
	"github.com/weids-dev/benchains/chaincodes/wrappers/plasma"
	"github.com/weids-dev/benchains/chaincodes/wrappers/zk"
)

func main() {
	// Initialize both contracts
	chaincode, err := contractapi.NewChaincode(&currency.CurrencyContract{}, &plasma.PlasmaContract{}, &zk.ZKContract{})
	// chaincode, err := contractapi.NewChaincode(&currency.CurrencyContract{})
	if err != nil {
		log.Panicf("Error creating chaincode: %v", err)
	}

	// Start the chaincode with both contracts
	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting chaincode: %v", err)
	}
}
