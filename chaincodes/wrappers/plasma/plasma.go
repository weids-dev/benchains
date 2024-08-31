package plasma

import (
	"fmt"
	"encoding/json"


	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// PlasmaContract for handling Plasma chain data
type PlasmaContract struct {
	contractapi.Contract
}

// CommitMerkleRoot commits the Merkle root of a Plasma block to the root chain
func (pc *PlasmaContract) CommitMerkleRoot(ctx contractapi.TransactionContextInterface, blockNumber string, merkleRoot string) error {
	err := ctx.GetStub().PutState(blockNumber, []byte(merkleRoot))
	if err != nil {
		return fmt.Errorf("failed to commit merkle root: %v", err)
	}
	return nil
}

// QueryMerkleRoot retrieves the Merkle root for a given Plasma block number
func (pc *PlasmaContract) QueryMerkleRoot(ctx contractapi.TransactionContextInterface, blockNumber string) (string, error) {
	merkleRootBytes, err := ctx.GetStub().GetState(blockNumber)
	if err != nil {
		return "", fmt.Errorf("failed to read merkle root data from world state: %v", err)
	}
	if merkleRootBytes == nil {
		return "", fmt.Errorf("no data found for block number: %s", blockNumber)
	}

	return string(merkleRootBytes), nil
}

// QueryAllMerkleRoots retrieves all Merkle roots committed to the root chain
func (pc *PlasmaContract) QueryAllMerkleRoots(ctx contractapi.TransactionContextInterface) (string, error) {
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return "", fmt.Errorf("failed to get state by range: %v", err)
	}
	defer resultsIterator.Close()

	var results []map[string]string
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return "", fmt.Errorf("failed to iterate through results: %v", err)
		}

		result := map[string]string{
			"BlockNumber": queryResponse.Key,
			"MerkleRoot":  string(queryResponse.Value),
		}
		results = append(results, result)
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results to JSON: %v", err)
	}

	return string(resultsJSON), nil
}

// PlasmaInitLedger initializes the ledger with an example block
func (pc *PlasmaContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	// Example data to initialize the ledger
	initialBlockNumber := "0"
	initialMerkleRoot := "0000000000000000000000000000000000000000000000000000000000000000"

	err := ctx.GetStub().PutState(initialBlockNumber, []byte(initialMerkleRoot))
	if err != nil {
		return fmt.Errorf("failed to initialize ledger: %v", err)
	}

	return nil
}
