package zk

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// ZKContract defines the smart contract for handling ZK-Rollups on Hyperledger Fabric Layer 1
type ZKContract struct {
	contractapi.Contract
}

// PublicInputs defines the public inputs for ZK proof verification, matching the operator's circuit
type PublicInputs struct {
	OldRoot *big.Int `gnark:"oldRoot,public"`
	NewRoot *big.Int `gnark:"newRoot,public"`
}

// Adjustable constants for ProofMerkleCircuit
const (
	D2 = 10 // ProofMerkleCircuit: Number of Leaves would be 2^10 = 1024
	B2 = 32 // Number of transactions in the batch
)

// ProofMerkleCircuit verifies a batch of transactions updating a Merkle tree sequentially.
type ProofMerkleCircuit struct {
	// Public inputs
	OldRoot frontend.Variable `gnark:"oldRoot,public"`
	NewRoot frontend.Variable `gnark:"newRoot,public"`

	// Private inputs: B2 transactions
	Transactions [B2]struct {
		OldName    frontend.Variable     `gnark:"oldName"`
		OldBalance frontend.Variable     `gnark:"oldBalance"`
		NewName    frontend.Variable     `gnark:"newName"`
		BenChange  frontend.Variable     `gnark:"benChange"` // Changed from NewBalance
		Siblings   [D2]frontend.Variable `gnark:"siblings"`
		PathBits   [D2]frontend.Variable `gnark:"pathBits"`
	}
}

// Define implements the circuit constraints.
func (c *ProofMerkleCircuit) Define(api frontend.API) error {
	var previousNewRoot frontend.Variable

	for k := 0; k < B2; k++ {
		tx := c.Transactions[k]

		// Compute old leaf hash: H_old_k = MiMC(OldName, OldBalance)
		mimcOld, err := mimc.NewMiMC(api)
		if err != nil {
			return err
		}
		mimcOld.Write(tx.OldName, tx.OldBalance)
		H_old_k := mimcOld.Sum()

		// Compute old root from H_old_k and Merkle proof
		currentHash := H_old_k
		for i := 0; i < D2; i++ {
			left := api.Select(tx.PathBits[i], currentHash, tx.Siblings[i])
			right := api.Select(tx.PathBits[i], tx.Siblings[i], currentHash)
			mimcLevel, err := mimc.NewMiMC(api)
			if err != nil {
				return err
			}
			mimcLevel.Write(left, right)
			currentHash = mimcLevel.Sum()
		}
		ComputedOldRoot_k := currentHash

		// Calculate NewBalance inside the circuit by adding OldBalance and BenChange
		NewBalance := api.Add(tx.OldBalance, tx.BenChange)

		// Compute new leaf hash: H_new_k = MiMC(NewName, NewBalance)
		mimcNew, err := mimc.NewMiMC(api)
		if err != nil {
			return err
		}
		mimcNew.Write(tx.NewName, NewBalance)
		H_new_k := mimcNew.Sum()

		// Compute new root from H_new_k and the same Merkle proof
		currentHash = H_new_k
		for i := 0; i < D2; i++ {
			left := api.Select(tx.PathBits[i], currentHash, tx.Siblings[i])
			right := api.Select(tx.PathBits[i], tx.Siblings[i], currentHash)
			mimcLevel, err := mimc.NewMiMC(api)
			if err != nil {
				return err
			}
			mimcLevel.Write(left, right)
			currentHash = mimcLevel.Sum()
		}
		ComputedNewRoot_k := currentHash

		// Chain the roots: old root matches previous new root (or OldRoot for k=0)
		if k == 0 {
			api.AssertIsEqual(ComputedOldRoot_k, c.OldRoot)
		} else {
			api.AssertIsEqual(ComputedOldRoot_k, previousNewRoot)
		}
		previousNewRoot = ComputedNewRoot_k

		// Ensure PathBits are boolean (0 or 1)
		for i := 0; i < D2; i++ {
			api.AssertIsBoolean(tx.PathBits[i])
		}
	}

	// Verify the final root matches NewRoot
	api.AssertIsEqual(previousNewRoot, c.NewRoot)

	return nil
}

// InitLedger initializes the chaincode with the verifying key and initial state root
func (c *ZKContract) InitLedger(ctx contractapi.TransactionContextInterface, verifyingKeyBase64 string, initialRootBase64 string) error {
	// Decode the verifying key from base64
	verifyingKeyBytes, err := base64.StdEncoding.DecodeString(verifyingKeyBase64)
	if err != nil {
		return fmt.Errorf("failed to decode verifying key: %v", err)
	}

	// Store the verifying key in the ledger state
	err = ctx.GetStub().PutState("verifyingKey", verifyingKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to store verifying key: %v", err)
	}

	// Set the initial state root for block 1 (similar to PlasmaContract)
	err = ctx.GetStub().PutState("stateRoot:1", []byte(initialRootBase64))
	if err != nil {
		return fmt.Errorf("failed to set initial state root: %v", err)
	}

	// Initialize the latest block number to 1
	err = ctx.GetStub().PutState("latestBlockNumber", []byte("1"))
	if err != nil {
		return fmt.Errorf("failed to set latest block number: %v", err)
	}

	return nil
}

// CommitNoChange commits a state root for a block with no state-changing transactions
func (c *ZKContract) CommitNoChange(ctx contractapi.TransactionContextInterface, blockId string, stateRootBase64 string) error {
	// Retrieve the latest committed block number
	latestBlockNumberBytes, err := ctx.GetStub().GetState("latestBlockNumber")
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %v", err)
	}
	if latestBlockNumberBytes == nil {
		return fmt.Errorf("latest block number not initialized")
	}
	latestBlockNumber, err := strconv.Atoi(string(latestBlockNumberBytes))
	if err != nil {
		return fmt.Errorf("invalid latest block number: %v", err)
	}

	// Convert blockId to integer and verify it's the next block
	blockIdInt, err := strconv.Atoi(blockId)
	if err != nil {
		return fmt.Errorf("invalid blockId: %v", err)
	}
	if blockIdInt != latestBlockNumber+1 {
		return fmt.Errorf("expected blockId %d, got %d", latestBlockNumber+1, blockIdInt)
	}

	// Get the previous state root (blockId - 1)
	prevBlockIdStr := strconv.Itoa(blockIdInt - 1)
	prevStateRootKey := "stateRoot:" + prevBlockIdStr
	prevStateRootBase64, err := ctx.GetStub().GetState(prevStateRootKey)
	if err != nil {
		return fmt.Errorf("failed to get previous state root for block %s: %v", prevBlockIdStr, err)
	}
	if prevStateRootBase64 == nil {
		return fmt.Errorf("previous state root not found for block %s", prevBlockIdStr)
	}

	// Verify that the submitted state root matches the previous state root
	if string(prevStateRootBase64) != stateRootBase64 {
		return fmt.Errorf("stateRoot does not match the previous state root for block %s", prevBlockIdStr)
	}

	// Store the state root for the current block
	newStateRootKey := "stateRoot:" + blockId
	err = ctx.GetStub().PutState(newStateRootKey, []byte(stateRootBase64))
	if err != nil {
		return fmt.Errorf("failed to store state root for block %s: %v", blockId, err)
	}

	// Update the latest block number
	err = ctx.GetStub().PutState("latestBlockNumber", []byte(blockId))
	if err != nil {
		return fmt.Errorf("failed to update latest block number: %v", err)
	}

	return nil
}

// CommitProof verifies a ZK proof and updates the state root if valid
func (c *ZKContract) CommitProof(ctx contractapi.TransactionContextInterface, blockId string, oldRootBase64 string, newRootBase64 string, proofBase64 string) error {
	// Retrieve the latest committed block number
	latestBlockNumberBytes, err := ctx.GetStub().GetState("latestBlockNumber")
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %v", err)
	}
	if latestBlockNumberBytes == nil {
		return fmt.Errorf("latest block number not initialized")
	}
	latestBlockNumber, err := strconv.Atoi(string(latestBlockNumberBytes))
	if err != nil {
		return fmt.Errorf("invalid latest block number: %v", err)
	}

	// Convert blockId to integer and verify it's the next block
	blockIdInt, err := strconv.Atoi(blockId)
	if err != nil {
		return fmt.Errorf("invalid blockId: %v", err)
	}
	if blockIdInt != latestBlockNumber+1 {
		return fmt.Errorf("expected blockId %d, got %d", latestBlockNumber+1, blockIdInt)
	}

	// Get the previous state root (blockId - 1)
	prevBlockIdStr := strconv.Itoa(blockIdInt - 1)
	prevStateRootKey := "stateRoot:" + prevBlockIdStr
	prevStateRootBase64, err := ctx.GetStub().GetState(prevStateRootKey)
	if err != nil {
		return fmt.Errorf("failed to get previous state root for block %s: %v", prevBlockIdStr, err)
	}
	if prevStateRootBase64 == nil {
		return fmt.Errorf("previous state root not found for block %s", prevBlockIdStr)
	}

	// Verify that oldRoot matches the previous state root
	if string(prevStateRootBase64) != oldRootBase64 {
		return fmt.Errorf("oldRoot does not match the state root of block %s", prevBlockIdStr)
	}

	// Decode oldRoot and newRoot from base64 to *big.Int for verification
	oldRootBytes, err := base64.StdEncoding.DecodeString(oldRootBase64)
	if err != nil {
		return fmt.Errorf("failed to decode oldRoot: %v", err)
	}
	oldRoot := new(big.Int).SetBytes(oldRootBytes)

	newRootBytes, err := base64.StdEncoding.DecodeString(newRootBase64)
	if err != nil {
		return fmt.Errorf("failed to decode newRoot: %v", err)
	}
	newRoot := new(big.Int).SetBytes(newRootBytes)

	// Decode and deserialize the proof
	proofBytes, err := base64.StdEncoding.DecodeString(proofBase64)
	if err != nil {
		return fmt.Errorf("failed to decode proof: %v", err)
	}
	proof, err := deserializeProof(proofBytes)
	if err != nil {
		return fmt.Errorf("failed to deserialize proof: %v", err)
	}

	// Retrieve and deserialize the verifying key
	verifyingKeyBytes, err := ctx.GetStub().GetState("verifyingKey")
	if err != nil {
		return fmt.Errorf("failed to get verifying key: %v", err)
	}
	if verifyingKeyBytes == nil {
		return fmt.Errorf("verifying key not initialized")
	}
	vk, err := deserializeVerifyingKey(verifyingKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to deserialize verifying key: %v", err)
	}

	var publicAssignment ProofMerkleCircuit
	publicAssignment.OldRoot = oldRoot
	publicAssignment.NewRoot = newRoot

	// Generate public witness
	publicWitness, err := frontend.NewWitness(&publicAssignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		return fmt.Errorf("failed to create public witness: %v", err)
	}

	// Verify the proof using Gnark
	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		return fmt.Errorf("proof verification failed: %v", err)
	}

	// Proof is valid, update the state
	// Store the new state root
	newStateRootKey := "stateRoot:" + blockId
	err = ctx.GetStub().PutState(newStateRootKey, []byte(newRootBase64))
	if err != nil {
		return fmt.Errorf("failed to store new state root for block %s: %v", blockId, err)
	}

	// Store the proof
	proofKey := "proof:" + blockId
	err = ctx.GetStub().PutState(proofKey, []byte(proofBase64))
	if err != nil {
		return fmt.Errorf("failed to store proof for block %s: %v", blockId, err)
	}

	// Update the latest block number
	err = ctx.GetStub().PutState("latestBlockNumber", []byte(blockId))
	if err != nil {
		return fmt.Errorf("failed to update latest block number: %v", err)
	}

	return nil
}

// QueryStateRoot retrieves the state root for a specific block
func (c *ZKContract) QueryStateRoot(ctx contractapi.TransactionContextInterface, blockId string) (string, error) {
	stateRootKey := "stateRoot:" + blockId
	stateRootBytes, err := ctx.GetStub().GetState(stateRootKey)
	if err != nil {
		return "", fmt.Errorf("failed to get state root for block %s: %v", blockId, err)
	}
	if stateRootBytes == nil {
		return "", fmt.Errorf("state root not found for block %s", blockId)
	}
	return string(stateRootBytes), nil
}

// QueryAllStateRoots retrieves all committed state roots
func (c *ZKContract) QueryAllStateRoots(ctx contractapi.TransactionContextInterface) (string, error) {
	// Get the latest block number
	latestBlockNumberBytes, err := ctx.GetStub().GetState("latestBlockNumber")
	if err != nil {
		return "", fmt.Errorf("failed to get latest block number: %v", err)
	}
	if latestBlockNumberBytes == nil {
		return "", fmt.Errorf("latest block number not initialized")
	}
	latestBlockNumber, err := strconv.Atoi(string(latestBlockNumberBytes))
	if err != nil {
		return "", fmt.Errorf("invalid latest block number: %v", err)
	}

	// Collect all state roots from block 0 to the latest block
	var results []map[string]string
	for i := 0; i <= latestBlockNumber; i++ {
		blockIdStr := strconv.Itoa(i)
		stateRootKey := "stateRoot:" + blockIdStr
		stateRootBytes, err := ctx.GetStub().GetState(stateRootKey)
		if err != nil {
			return "", fmt.Errorf("failed to get state root for block %s: %v", blockIdStr, err)
		}
		if stateRootBytes != nil {
			result := map[string]string{
				"BlockNumber": blockIdStr,
				"StateRoot":   string(stateRootBytes),
			}
			results = append(results, result)
		}
	}

	// Marshal results to JSON
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state roots to JSON: %v", err)
	}

	return string(resultsJSON), nil
}

// deserializeProof converts proof bytes back to a groth16.Proof object
func deserializeProof(proofBytes []byte) (groth16.Proof, error) {
	proof := groth16.NewProof(ecc.BN254)
	_, err := proof.ReadFrom(bytes.NewReader(proofBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize proof: %v", err)
	}
	return proof, nil
}

// deserializeVerifyingKey converts verifying key bytes back to a groth16.VerifyingKey object
func deserializeVerifyingKey(vkBytes []byte) (groth16.VerifyingKey, error) {
	vk := groth16.NewVerifyingKey(ecc.BN254)
	_, err := vk.ReadFrom(bytes.NewReader(vkBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize verifying key: %v", err)
	}
	return vk, nil
}
