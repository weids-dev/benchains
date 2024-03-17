package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	"github.com/weids-dev/benchains/chaincodes/wrappers/currency"
)

func main() {
	currencyChaincode, err := contractapi.NewChaincode(&currency.CurrencyContract{})
	if err != nil {
		log.Panicf("Error creating currency chaincode: %v", err)
	}

	if err := currencyChaincode.Start(); err != nil {
		log.Panicf("Error starting currency chaincode: %v", err)
	}
}
