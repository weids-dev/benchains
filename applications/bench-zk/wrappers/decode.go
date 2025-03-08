// wrappers/decode.go

package wrappers

import (
	"bytes"
	"os/exec"
	"context"
	"fmt"
	"encoding/json"
	"encoding/base64"
	"log"
	"errors"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"google.golang.org/grpc/status"
	ggateway "github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"bench-zk/merkle"
)


// -------------------------------------------------------------
// Helper functions below
// -------------------------------------------------------------

func decodeChainInfo(chainInfoData []byte) (string, error) {
	// Prepare command to decode chain info data using configtxlator
	cmd := exec.Command("configtxlator", "proto_decode", "--type", "common.BlockchainInfo")
	cmd.Stdin = bytes.NewReader(chainInfoData)

	// Run the command and capture output
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute configtxlator command: %w", err)
	}

	return string(stdout), nil
}

// Function to decode block data using configtxlator
func decodeBlock(blockData []byte) (string, error) {
	// Prepare command to decode block data using configtxlator
	cmd := exec.Command("configtxlator", "proto_decode", "--type", "common.Block")
	cmd.Stdin = bytes.NewReader(blockData)

	// Run the command and capture output
	stdout, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute configtxlator command: %w", err)
	}

	return string(stdout), nil
}

func extractNewestBlockNumber(decodedChainInfo string) (uint64, error) {
	// Define a struct to hold the decoded chain info data
	var chainInfo map[string]interface{}
	err := json.Unmarshal([]byte(decodedChainInfo), &chainInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to parse decoded chain info data: %w", err)
	}

	log.Println("\n ChainInfo: ", chainInfo)

	// Get the blockchain height
	height, ok := chainInfo["height"].(string) // Block height is stored as a string in JSON
	if !ok {
		return 0, fmt.Errorf("failed to find height in chain info")
	}

	// Convert the height to uint64
	var heightUint uint64
	fmt.Sscanf(height, "%d", &heightUint)

	// Newest block number is height - 1
	if heightUint == 0 {
		return 0, fmt.Errorf("chain height is zero, no blocks in the chain")
	}

	newestBlockNumber := heightUint - 1

	return newestBlockNumber, nil
}


func getBlockByNumber(contract *client.Contract, channelName string, number string) []byte {
	log.Println("\n--> Evaluate Transaction: getBlock from system chaincode qscc GetBlockByNumber")

	evaluateResult, err := contract.EvaluateTransaction("GetBlockByNumber", channelName, number)
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	return evaluateResult
}

// Extract transactions from the decoded block
func extractTransactions(decodedBlock string) ([]merkle.TransactionData, error) {
	// Define a struct to hold the decoded block data
	var blockData map[string]interface{}
	err := json.Unmarshal([]byte(decodedBlock), &blockData)
	fmt.Printf("*** Decoded Block: %s\n", decodedBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decoded block data: %w", err)
	}

	// Navigate the JSON structure to extract transactions
	data, ok := blockData["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find data field in block")
	}

	dataArray, ok := data["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find data array in block data")
	}

	var transactions []merkle.TransactionData
	for _, item := range dataArray {
		envelope, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to parse transaction envelope")
		}

		payload, ok := envelope["payload"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to find payload in transaction envelope")
		}

		channelHeader, ok := payload["header"].(map[string]interface{})["channel_header"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to find channel header in transaction payload")
		}

		transactionID, ok := channelHeader["tx_id"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to find transaction ID")
		}

		// Extract writes from the transaction payload
		transaction, err := extractTransaction(payload)

		if err != nil {
			return nil, fmt.Errorf("failed to extract writes: %w", err)
		}

		transactions = append(transactions, merkle.TransactionData{
			TxID:   transactionID,
			Args:   transaction,
		})
	}
	return transactions, nil
}

// Extract transaction from the transaction payload
func extractTransaction(payload map[string]interface{}) ([]string, error) {
	// Navigate to the 'data' field under 'payload'
	data, ok := payload["data"].(map[string]interface{})
	
	// fmt.Printf("*** Payload: %s\n", payload)

	if !ok {
		return nil, fmt.Errorf("failed to find data field in payload")
	}

	// Traverse further into the 'actions' field to find the writes
	actions, ok := data["actions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to find actions in transaction payload")
	}

	var args []string
	for _, action := range actions {
		actionData, ok := action.(map[string]interface{})
		if !ok {
			fmt.Println("Error: Input Args 1")
		}

		// Navigate to 'payload' and then to 'chaincode_proposal_payload'
		chaincodeActionPayload, ok := actionData["payload"].(map[string]interface{})
		if !ok {
			fmt.Println("Error: Input Args 2")
		}

		chaincodeProposalPayload, ok := chaincodeActionPayload["chaincode_proposal_payload"].(map[string]interface{})
		if !ok {
			fmt.Println("Error: Input Args 4")
		}

		input, ok := chaincodeProposalPayload["input"].(map[string]interface{})
		if !ok {
			fmt.Println("Error: Input Args 5")
		}

		spec, ok := input["chaincode_spec"].(map[string]interface{})
		if !ok {
			fmt.Println("Error: Input Args 6")
		}

		inputArgs, ok := spec["input"].(map[string]interface{})
		if !ok {
			fmt.Println("Error: Input Args 7")
		}

		iargs, ok := inputArgs["args"].([]interface{})
		if !ok {
			fmt.Println("Error: Input Args 8")
		}

		fmt.Printf("Input Args: %s\n", iargs)
		for _, iarg := range iargs {
			arg, ok := iarg.(string)
			if !ok {
				log.Fatal("Expected a string in the args array")
			}
			decoded, err := base64.StdEncoding.DecodeString(arg)
			if err != nil {
				log.Fatal("Error decoding base64 string:", err)
			}
			args = append(args, string(decoded))
		}
	}
	return args, nil
}


// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}

// Submit transaction, passing in the wrong number of arguments ,expected to throw an error containing details of any error responses from the smart contract.
func errorHandling(contract *client.Contract, err error) {
	switch err := err.(type) {
	case *client.EndorseError:
		fmt.Printf("Endorse error for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
	case *client.SubmitError:
		fmt.Printf("Submit error for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
	case *client.CommitStatusError:
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Timeout waiting for transaction %s commit status: %s", err.TransactionID, err)
		} else {
			fmt.Printf("Error obtaining commit status for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
		}
	case *client.CommitError:
		fmt.Printf("Transaction %s failed to commit with status %d: %s\n", err.TransactionID, int32(err.Code), err)
	default:
		panic(fmt.Errorf("unexpected error type %T: %w", err, err))
	}

	// Any error that originates from a peer or orderer node external to the gateway will have its details
	// embedded within the gRPC status error. The following code shows how to extract that.
	statusErr := status.Convert(err)

	details := statusErr.Details()
	if len(details) > 0 {
		fmt.Println("Error Details:")

		for _, detail := range details {
			switch detail := detail.(type) {
			case *ggateway.ErrorDetail:
				fmt.Printf("- address: %s, mspId: %s, message: %s\n", detail.Address, detail.MspId, detail.Message)
			}
		}
	}
}

