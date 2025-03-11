package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"net/http"
	"strings"
)

type PeerConfig struct {
	MSPID         string
	CryptoPath    string
	CertPath      string
	KeyPath       string
	TLSCertPath   string
	PeerEndpoint  string
	GatewayPeer   string
	ChannelName   string
	ChaincodeName string
}

// TransactionData holds the transaction ID and its corresponding writes
type TransactionData struct {
	TxID   string
	Writes []map[string]interface{}
}

// UserState

// Write represents a simplified write structure
type Write struct {
	Key   string
	Value string
}

// computeHash computes the SHA-256 hash of the input data
func computeHash(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// buildMerkleTree builds a Merkle tree from the given transactions and returns the Merkle root
func buildMerkleTree(transactions []TransactionData) string {
	if len(transactions) == 0 {
		return ""
	}

	var leaves [][]byte
	for _, tx := range transactions {
		txHash := computeHash([]byte(tx.TxID)) // TODO: encode TxID + Writes on Merkle leaf
		leaves = append(leaves, txHash)
	}

	for len(leaves) > 1 {
		var newLevel [][]byte
		for i := 0; i < len(leaves); i += 2 {
			if i+1 < len(leaves) {
				combined := append(leaves[i], leaves[i+1]...)
				newLevel = append(newLevel, computeHash(combined))
			} else {
				newLevel = append(newLevel, leaves[i])
			}
		}
		leaves = newLevel
	}

	return hex.EncodeToString(leaves[0])
}

// Item represents an in-game item with a name, type, and value.
// It is used to manage the inventory items of a player.
type Item struct {
	Name  string `json:"name"`  // Name is the item's unique identifier.
	Type  string `json:"type"`  // Type categorizes the item.
	Value int    `json:"value"` // Value represents the item's worth or power.
}

// Player represents a game player with a unique ID, balance, and inventory of items.
// It encapsulates the player's state within the game.
type Player struct {
	ID      string  `json:"id"`      // ID is the player's unique identifier.
	Balance float64 `json:"balance"` // Balance tracks the currency the player has.
	Items   []Item  `json:"items"`   // Items hold the collection of items owned by the player.
}

// BankTransaction represents a transaction from the bank to buy in-game currency.
type BankTransaction struct {
	UserID        string  `json:"userID"`
	AmountUSD     float64 `json:"amountUSD"`
	TransactionID string  `json:"transactionID"`
}

var now = time.Now()
var playerId = fmt.Sprintf("player%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)

// TODO: change the rate as the time goes
const rate string = "0.313"

func plasmaHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		if r.Method == http.MethodGet {
			queryAllMerkleRoots(contract)
			return
		}
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}
}

func depositHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract txID, USD, and playerID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 5 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	transactionId := parts[2]
	USD := parts[3]
	playerId := parts[4]

	log.Printf("Depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)
	recordBankTransaction(contract, playerId, USD, transactionId)
	log.Printf("Finish depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)
}

func exchangeHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract txID and playerID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	transactionId := parts[2]
	playerId := parts[3]

	log.Printf("Exchanging in-game currency for player %s with txID %s\n", playerId, transactionId)
	exchangeInGameCurrency(contract, playerId, transactionId, rate)
	log.Printf("finish exchanging in-game currency for player %s with txID %s and rate %s\n", playerId, transactionId, rate)
}

func bankExchangeHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract txID, USD and playerID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 5 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	transactionId := parts[2]
	USD := parts[3]
	playerId := parts[4]

	log.Printf("Depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)
	recordBankTransaction(contract, playerId, USD, transactionId)
	log.Printf("Finish depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)

	log.Printf("Exchanging in-game currency for player %s with txID %s\n", playerId, transactionId)
	exchangeInGameCurrency(contract, playerId, transactionId, rate)
	log.Printf("finish exchanging in-game currency for player %s with txID %s and rate %s\n", playerId, transactionId, rate)
}

func createPlayerHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		if r.Method == http.MethodGet {
			getAllPlayers(contract)
			getPlayersNum(contract)
			return
		}
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	playerId := parts[2]

	log.Printf("Creating player with ID: %s\n", playerId)
	createPlayer(contract, playerId)
	log.Printf("PUT request processed for playerId: %s", playerId)
}

var debug = true // Set this to true to enable logging

func main() {
	if debug {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(io.Discard)
	}

	plasmaChainConfig := PeerConfig{
		MSPID:         "org02MSP",
		CryptoPath:    "../../networks/fabric/certs/chains/peerOrganizations/org02.chains",
		CertPath:      "../../networks/fabric/certs/chains/peerOrganizations/org02.chains/users/User1@org02.chains/msp/signcerts/User1@org02.chains-cert.pem",
		KeyPath:       "../../networks/fabric/certs/chains/peerOrganizations/org02.chains/users/User1@org02.chains/msp/keystore/",
		TLSCertPath:   "../../networks/fabric/certs/chains/peerOrganizations/org02.chains/peers/peer1.org02.chains/tls/ca.crt",
		PeerEndpoint:  "localhost:6002",
		GatewayPeer:   "peer1.org02.chains",
		ChannelName:   "chains02",
		ChaincodeName: "pasic",
	}

	rootChainConfig := PeerConfig{
		MSPID:         "org01MSP",
		CryptoPath:    "../../networks/fabric/certs/chains/peerOrganizations/org01.chains",
		CertPath:      "../../networks/fabric/certs/chains/peerOrganizations/org01.chains/users/User1@org01.chains/msp/signcerts/User1@org01.chains-cert.pem",
		KeyPath:       "../../networks/fabric/certs/chains/peerOrganizations/org01.chains/users/User1@org01.chains/msp/keystore/",
		TLSCertPath:   "../../networks/fabric/certs/chains/peerOrganizations/org01.chains/peers/peer1.org01.chains/tls/ca.crt",
		PeerEndpoint:  "localhost:6001",
		GatewayPeer:   "peer1.org01.chains",
		ChannelName:   "chains",
		ChaincodeName: "basic",
	}

	// Establish connection with plasma chain
	// The gRPC client connection should be shared by all Gateway connections to this endpoint
	plasma_clientConnection := newGrpcConnection(plasmaChainConfig)
	defer plasma_clientConnection.Close()

	plasma_id := newIdentity(plasmaChainConfig)
	plasma_sign := newSign(plasmaChainConfig)

	// Create a Gateway connection for a specific client identity
	plasma_gw, err := client.Connect(
		plasma_id,
		client.WithSign(plasma_sign),
		client.WithClientConnection(plasma_clientConnection),
		// Default timeouts for different gRPC calls
		client.WithEvaluateTimeout(1*time.Minute),
		client.WithEndorseTimeout(1*time.Minute),
		client.WithSubmitTimeout(1*time.Minute),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer plasma_gw.Close()

	plasma_network := plasma_gw.GetNetwork(plasmaChainConfig.ChannelName)
	plasma_contract := plasma_network.GetContract(plasmaChainConfig.ChaincodeName)
	syscontract := plasma_network.GetContract("qscc") // system chaincode

	initLedger(plasma_contract)
	getAllPlayers(plasma_contract)

	// Establish connection with main chain
	root_clientConnection := newGrpcConnection(rootChainConfig)
	defer root_clientConnection.Close()

	root_id := newIdentity(rootChainConfig)
	root_sign := newSign(rootChainConfig)

	// Create a Gateway connection for a specific client identity
	root_gw, err := client.Connect(
		root_id,
		client.WithSign(root_sign),
		client.WithClientConnection(root_clientConnection),
		// Default timeouts for different gRPC calls
		client.WithEvaluateTimeout(1*time.Minute),
		client.WithEndorseTimeout(1*time.Minute),
		client.WithSubmitTimeout(1*time.Minute),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer root_gw.Close()

	root_network := root_gw.GetNetwork(rootChainConfig.ChannelName)
	root_contract := root_network.GetContract(rootChainConfig.ChaincodeName)

	initLedger2(root_contract)
	getAllPlayers(root_contract)

	/*
	   Check and Commit Periodically
	*/

	newestCommittedBlockNumber := uint64(1) // Initially, no blocks committed
	// Periodic loop to check the newest block and fetch new blocks
	ticker := time.NewTicker(5 * time.Second) // 5 seconds interval
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			// Step 1: Get the newest block number
			newestBlockNumber, err := getNewestBlockNumber(syscontract, "chains02")

			if err != nil {
				log.Println("Error getting newest block number:", err)
				continue
			}

			fmt.Printf("Newest block number: ", newestBlockNumber)
			fmt.Printf(" || Newest committed block number: %s \n", newestCommittedBlockNumber)

			// Step 2: Check if there are new blocks to process
			if newestBlockNumber > newestCommittedBlockNumber {
				fmt.Printf("Found new blocks to commit: %d to %d    || ", newestCommittedBlockNumber+1, newestBlockNumber)

				// Step 3: Process all blocks between newestCommittedBlockNumber + 1 and newestBlockNumber
				for blockNumber := newestCommittedBlockNumber + 1; blockNumber <= newestBlockNumber; blockNumber++ {
					snum := strconv.FormatUint(blockNumber, 10)

					blockBytes := getBlockByNumber(syscontract, "chains02", snum)
					block, err := decodeBlock(blockBytes)
					if err != nil {
						fmt.Println("Error decoding block:", err)
						continue
					}

					transactions, err := extractTransactions(block)
					if err != nil {
						fmt.Println("Error extracting transactions:", err)
						continue
					}

					if err != nil {
						fmt.Println("Error extracting transaction:", err)
						continue
					}

					// Output the extracted transactions
					for _, tx := range transactions {
						log.Printf("TxID: %s\n", tx.TxID)
						for _, write := range tx.Writes {
							log.Printf("Write: %+v\n", write)
						}
					}

					fmt.Printf("Number of transactions in this block: %d   || ", len(transactions))

					// Compute the Merkle root of the transactions
					merkleRoot := buildMerkleTree(transactions)

					// Commit the Merkle root to the root chain
					go commitMerkleRoot(root_contract, snum, merkleRoot)

					fmt.Printf("Committed Merkle root for block %d: %s\n", blockNumber, merkleRoot)
				}

				// Step 4: Update newest committed block number after processing
				newestCommittedBlockNumber = newestBlockNumber
				// queryAllMerkleRoots(root_contract)
			}
		}
	}()

	/*
	   Test Functions Begin
	*/

	time.Sleep(10 * time.Second)

	// All those will be written to the ledger
	go createPlayer(plasma_contract, "AWANG01")
	go createPlayer(plasma_contract, "AWANG02")
	go createPlayer(plasma_contract, "AWANG03")
	go createPlayer(plasma_contract, "AWANG04")
	go createPlayer(plasma_contract, "AWANG05")
	go createPlayer(plasma_contract, "AWANG06")
	go createPlayer(plasma_contract, "AWANG07")
	go createPlayer(plasma_contract, "AWANG08")

	time.Sleep(10 * time.Second)

	go recordBankTransaction(plasma_contract, "AWANG01", "1", "TXXAWANG01")
	go recordBankTransaction(plasma_contract, "AWANG02", "2", "TXXAWANG02")
	go recordBankTransaction(plasma_contract, "AWANG03", "3", "TXXAWANG03")
	go recordBankTransaction(plasma_contract, "AWANG04", "3", "TXXAWANG04")
	go recordBankTransaction(plasma_contract, "AWANG05", "3", "TXXAWANG05")
	go recordBankTransaction(plasma_contract, "AWANG06", "3", "TXXAWANG06")
	go recordBankTransaction(plasma_contract, "AWANG07", "3", "TXXAWANG07")
	go recordBankTransaction(plasma_contract, "AWANG08", "8", "TXXAWANG08")

	time.Sleep(10 * time.Second)

	go exchangeInGameCurrency(plasma_contract, "AWANG01", "TXXAWANG01", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG02", "TXXAWANG02", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG03", "TXXAWANG03", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG04", "TXXAWANG04", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG05", "TXXAWANG05", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG06", "TXXAWANG06", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG07", "TXXAWANG07", rate)
	go exchangeInGameCurrency(plasma_contract, "AWANG08", "TXXAWANG08", rate)

	time.Sleep(5 * time.Second)

	newestBlockNumber, err := getNewestBlockNumber(syscontract, "chains02")
	if err != nil {
		fmt.Println("Error getting newest block number:", err)
		return
	}

	log.Println("Newest Block Number:", newestBlockNumber)
	snum := strconv.FormatUint(newestBlockNumber, 10)

	blockBytes := getBlockByNumber(syscontract, "chains02", snum)
	block, err := decodeBlock(blockBytes)
	if err != nil {
		panic(fmt.Errorf("failed to decode block: %w", err))
	}

	// fmt.Printf("%s\n", block)

	transactions, err := extractTransactions(block)

	if err != nil {
		fmt.Println("Error extracting transactions:", err)
		return
	}

	// fmt.Println(transactions)

	// Output the extracted transactions
	for _, tx := range transactions {
		log.Printf("TxID: %s\n", tx.TxID)
		for _, write := range tx.Writes {
			log.Printf("Write: %+v\n", write)
		}
	}

	/*
	   Test Functions End
	*/

	http.HandleFunc("/player/", func(w http.ResponseWriter, r *http.Request) {
		createPlayerHandler(w, r, plasma_contract)
	})
	http.HandleFunc("/bank/", func(w http.ResponseWriter, r *http.Request) {
		depositHandler(w, r, plasma_contract)
	})
	http.HandleFunc("/exchange/", func(w http.ResponseWriter, r *http.Request) {
		exchangeHandler(w, r, plasma_contract)
	})
	http.HandleFunc("/bexchange/", func(w http.ResponseWriter, r *http.Request) {
		bankExchangeHandler(w, r, plasma_contract)
	})
	http.HandleFunc("/plasma/", func(w http.ResponseWriter, r *http.Request) {
		plasmaHandler(w, r, root_contract)
	})

	if err := http.ListenAndServe(":10809", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	fmt.Println("Server is listening on port 10809")
}

func newGrpcConnection(config PeerConfig) *grpc.ClientConn {
	certificate, err := loadCertificate(config.TLSCertPath)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, config.GatewayPeer)

	connection, err := grpc.Dial(config.PeerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}

	return connection
}

func newIdentity(config PeerConfig) *identity.X509Identity {
	certificate, err := loadCertificate(config.CertPath)
	if err != nil {
		panic(err)
	}

	id, err := identity.NewX509Identity(config.MSPID, certificate)
	if err != nil {
		panic(err)
	}

	return id
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	certificatePEM, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(certificatePEM)
}

func newSign(config PeerConfig) identity.Sign {
	files, err := os.ReadDir(config.KeyPath)
	if err != nil {
		panic(fmt.Errorf("failed to read private key directory: %w", err))
	}
	privateKeyPEM, err := os.ReadFile(path.Join(config.KeyPath, files[0].Name()))

	if err != nil {
		panic(fmt.Errorf("failed to read private key file: %w", err))
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		panic(err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		panic(err)
	}

	return sign
}

func commitMerkleRoot(contract *client.Contract, blockNumber, merkleRoot string) {
	log.Printf("\n--> Submit Transaction: CommitMerkleRoot \n")

	_, err := contract.SubmitTransaction("PlasmaContract:CommitMerkleRoot", blockNumber, merkleRoot)
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

// This type of transaction would typically only be run once by an application the first time it was started after its
// initial deployment. A new version of the chaincode deployed later would likely not need to run an "init" function.
//
// SubmitTransaction will submit a transaction to the ledger and return its result only after it is committed to the ledger.
// The transaction function will be evaluated on endorsing peers and then submitted to the ordering service to be committed to the ledger.
func initLedger(contract *client.Contract) {
	log.Printf("\n--> Submit Transaction: InitLedger, function creates the initial set of players on the ledger \n")

	_, err := contract.SubmitTransaction("CurrencyContract:InitLedger")

	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

// TODO: Seperate pasic and basic contract
func initLedger2(contract *client.Contract) {
	log.Printf("\n--> Submit Transaction: InitLedger, function creates the initial set of players on the ledger \n")

	_, err := contract.SubmitTransaction("CurrencyContract:InitLedger")

	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")

	log.Printf("\n--> Submit Transaction: InitLedger on PlasmaContract \n")

	_, err = contract.SubmitTransaction("PlasmaContract:InitLedger")

	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

func queryAllMerkleRoots(contract *client.Contract) {
	result, err := contract.EvaluateTransaction("PlasmaContract:QueryAllMerkleRoots")
	if err != nil {
		fmt.Printf("Failed to evaluate transaction: %v\n", err)
		return
	}

	fmt.Printf("All committed Merkle roots:\n%s\n", string(result))
}

// Evaluate a transaction to query ledger state.
func getAllPlayers(contract *client.Contract) {
	log.Println("\n--> Evaluate Transaction: GetAllPlayers, function returns all the current players on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("CurrencyContract:GetAllPlayers")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Records: %s\n", result)
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

// Extract transactions from the decoded block
func extractTransactions(decodedBlock string) ([]TransactionData, error) {
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

	var transactions []TransactionData
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
		writes, err := extractWrites(payload)

		_, err = extractTransaction(payload) // test

		if err != nil {
			return nil, fmt.Errorf("failed to extract writes: %w", err)
		}

		transactions = append(transactions, TransactionData{
			TxID:   transactionID,
			Writes: writes,
		})
	}

	return transactions, nil
}

// Extract writes from the transaction payload
func extractWrites(payload map[string]interface{}) ([]map[string]interface{}, error) {
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

	var writes []map[string]interface{}
	for _, action := range actions {
		actionData, ok := action.(map[string]interface{})
		if !ok {
			continue
		}

		// Navigate to 'payload' and then to 'chaincode_proposal_payload'
		chaincodeActionPayload, ok := actionData["payload"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract 'action' field to get the writes in the 'rwset'
		chaincodeAction, ok := chaincodeActionPayload["action"].(map[string]interface{})
		if !ok {
			continue
		}

		proposalResponsePayload, ok := chaincodeAction["proposal_response_payload"].(map[string]interface{})
		if !ok {
			continue
		}

		extension, ok := proposalResponsePayload["extension"].(map[string]interface{})
		if !ok {
			continue
		}

		results, ok := extension["results"].(map[string]interface{})
		if !ok {
			continue
		}

		nsRwset, ok := results["ns_rwset"].([]interface{})
		if !ok {
			continue
		}

		// Extract writes for each namespace read-write set (nsRwset)
		for _, rw := range nsRwset {
			rwset, ok := rw.(map[string]interface{})
			if !ok {
				continue
			}

			// Check for the 'rwset' field that contains the 'writes'
			rwsetData, ok := rwset["rwset"].(map[string]interface{})
			if !ok {
				continue
			}

			writeSet, ok := rwsetData["writes"].([]interface{})
			if !ok {
				continue
			}

			// Convert each write to a map and append to the list of writes
			for _, write := range writeSet {
				writeMap, ok := write.(map[string]interface{})
				if !ok {
					continue
				}
				writes = append(writes, writeMap)
			}
		}
	}

	return writes, nil
}

// Extract transaction from the transaction payload
func extractTransaction(payload map[string]interface{}) ([]map[string]interface{}, error) {
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

		args, ok := inputArgs["args"].([]interface{})
		if !ok {
			fmt.Println("Error: Input Args 8")
		}

		fmt.Printf("Input Args: %s\n", args)
	}

	return nil, nil
}

func getNewestBlockNumber(contract *client.Contract, channelName string) (uint64, error) {
	log.Println("\n--> Evaluate Transaction: getChainInfo from system chaincode qscc GetChainInfo")

	// Call QSCC to get the chain info of the specified channel
	evaluateResult, err := contract.EvaluateTransaction("GetChainInfo", channelName)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate transaction: %w", err)
	}

	// Decode the returned protobuf data to JSON
	chainInfoJSON, err := decodeChainInfo(evaluateResult)
	if err != nil {
		return 0, fmt.Errorf("failed to decode chain info: %w", err)
	}

	// Extract the height and compute the newest block number
	newestBlockNumber, err := extractNewestBlockNumber(chainInfoJSON)
	if err != nil {
		return 0, fmt.Errorf("failed to extract newest block number: %w", err)
	}

	return newestBlockNumber, nil
}

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

// getPlayersNum evaluates a transaction to query ledger state and prints the number of players
func getPlayersNum(contract *client.Contract) {
	log.Println("\n--> Evaluate Transaction: getPlayersNum, function returns the number of current players on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("CurrencyContract:GetAllPlayers")
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	// Unmarshal the JSON bytes into a slice of player structs
	var players []*Player
	if err := json.Unmarshal(evaluateResult, &players); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}

	// Now you can accurately get the number of players
	log.Printf("*** Number of Records: %d\n", len(players))
}

// createPlayer directly create a player with all attr initialized default
func createPlayer(contract *client.Contract, playerId string) {
	log.Printf("\n--> Submit Transaction: CreatePlayer \n")

	_, err := contract.SubmitTransaction("CurrencyContract:CreatePlayer", playerId)
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

// recordBankTransaction records a new bank transaction to the ledger
func recordBankTransaction(contract *client.Contract, userID, amountUSDStr, transactionID string) {
	log.Printf("\n--> Submit Transaction: RecordBankTransaction \n")

	_, err := contract.SubmitTransaction("CurrencyContract:RecordBankTransaction", userID, amountUSDStr, transactionID)

	if err != nil {
		// errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

// exchangeInGameCurrency let users to exchange their deposited USD to in-game currency
func exchangeInGameCurrency(contract *client.Contract, userID, transactionID, exchangeRateStr string) {
	log.Printf("\n--> Submit Transaction: ExchangeInGameCurrency \n")

	_, err := contract.SubmitTransaction("CurrencyContract:ExchangeInGameCurrency", userID, transactionID, exchangeRateStr)

	if err != nil {
		// errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
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
			case *gateway.ErrorDetail:
				fmt.Printf("- address: %s, mspId: %s, message: %s\n", detail.Address, detail.MspId, detail.Message)
			}
		}
	}
}

// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}
