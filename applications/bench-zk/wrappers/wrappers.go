// wrappers/wrappers.go
package wrappers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"time"

	"bench-zk/circuit"
	"bench-zk/gateway"
	"bench-zk/merkle"

	"github.com/hyperledger/fabric-gateway/pkg/client"
)

// The Operator wiil use UserState root as input to generate proof for exchangeBen
// The Operator will use Deposit root as input to generate proof for depositTransaction
type Wrappers struct {
	UserStates     []merkle.UserState
	Transactions   []merkle.TransactionData
	StateRoots     []string         // set of intermediate states between each transactions
	StateProofs    []merkle.MProof  // each Merkle proof to show that the state is exactly in the tree root
	Gw1            *gateway.Gateway // Gw1 represents the way operator communicate with Layer 1
	Gw2            *gateway.Gateway // Gw2 represents the way operator communicate with Layer 2
	LatestRoot     string           // The latest root committed to Layer 1
	LatestRootHash string           // The latest root hash committed to Layer 1
}

// NewWrappers initializes a new Wrappers instance.
// It receives two hain configurations to initialize Gw1 and Gw2,
// and initializes UserStates and Deposits as empty slices.
func NewWrappers(chain1, chain2 gateway.Chain) (*Wrappers, error) {
	// Initialize Gw1
	gw1, err := gateway.NewGateway(chain1)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Gw1: %w", err)
	}

	// Initialize Gw2
	gw2, err := gateway.NewGateway(chain2)
	if err != nil {
		gw1.Close() // Ensure Gw1 is closed if Gw2 initialization fails
		return nil, fmt.Errorf("failed to initialize Gw2: %w", err)
	}
	// TODO: Initialize ZK Circuits

	// Initialize Wrappers with empty UserStates and Deposits
	return &Wrappers{
		UserStates:     []merkle.UserState{},
		Transactions:   []merkle.TransactionData{},
		StateRoots:     []string{},        // set of intermediate states between each transactions
		StateProofs:    []merkle.MProof{}, // each Merkle proof to show that the state is exactly in the tree root.
		Gw1:            gw1,
		Gw2:            gw2,
		LatestRoot:     "",
		LatestRootHash: "",
	}, nil
}

// Close gracefully closes both gateways within Wrappers.
func (w *Wrappers) Close() error {
	if w.Gw1 != nil {
		w.Gw1.Close()
	}

	if w.Gw2 != nil {
		w.Gw2.Close()
	}

	return nil
}

func (w *Wrappers) Operate(ctx context.Context) error {
	newestCommittedBlockNumber := uint64(1)   // Initially, no blocks committed
	ticker := time.NewTicker(5 * time.Second) // 5 seconds interval
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Operator stopped due to context cancellation")
			return nil
		case <-ticker.C:
			// Step 1: Get the newest block number from Layer 2
			syscontract := w.Gw2.Gateway.GetNetwork(w.Gw2.ChannelName).GetContract("qscc")
			newestBlockNumber, err := getNewestBlockNumber(syscontract, w.Gw2.ChannelName)
			if err != nil {
				log.Println("Error getting newest block number:", err)
				continue
			}

			fmt.Printf("Newest block number: %d || Newest committed block number: %d\n", newestBlockNumber, newestCommittedBlockNumber)

			// Step 2: Check if there are new blocks to process
			if newestBlockNumber > newestCommittedBlockNumber {
				fmt.Printf("Found new blocks to commit: %d to %d    || ", newestCommittedBlockNumber+1, newestBlockNumber)

				// Step 3: Process all new blocks
				for blockNumber := newestCommittedBlockNumber + 1; blockNumber <= newestBlockNumber; blockNumber++ {
					snum := strconv.FormatUint(blockNumber, 10)
					blockBytes := getBlockByNumber(syscontract, w.Gw2.ChannelName, snum)
					block, err := decodeBlock(blockBytes) // Using decode package
					if err != nil {
						fmt.Println("Error decoding block:", err)
						continue
					}

					w.Transactions, err = extractTransactions(block)
					if err != nil {
						fmt.Println("Error extracting transactions:", err)
						continue
					}

					// Pretty print the extracted transactions
					for _, tx := range w.Transactions {
						log.Printf("TxID: %s", tx.TxID)
						for i, arg := range tx.Args {
							log.Printf("  Arg %d: %+v", i, arg)
						}
					}
					fmt.Printf("Number of transactions in this block: %d   || ", len(w.Transactions))

					// Process transactions before computing Merkle root
					err = w.processTransactions(w.Transactions, blockNumber)
					if err != nil {
						fmt.Printf("Error processing transactions: %v\n", err)
						continue
					}

					// Step 4: Compute the Merkle root using the merkle package
					transactionRoot := merkle.BuildMerkleTransactions(w.Transactions)
					merkleRoot := merkle.MerkleRootToBase64(transactionRoot)

					// Step 5: Commit the Merkle root to Layer 1
					go commitMerkleRoot(w.Gw1.Contract, snum, merkleRoot)
					fmt.Printf("Committed Merkle root for block %d: %s\n", blockNumber, merkleRoot)
				}
				newestCommittedBlockNumber = newestBlockNumber
			}
		}
	}
}

func (w *Wrappers) processTransactions(transactions []merkle.TransactionData, blockNumber uint64) error {
	if len(transactions) == 0 {
		return nil
	}

	// If this is the first transaction block, initialize our state
	if len(w.UserStates) == 0 {
		// Initialize with dummy user states up to 2^D2 (Merkle tree capacity)
		maxUsers := 1 << circuit.D2 // 2^10 = 1024 users
		for i := 0; i < maxUsers; i++ {
			nameInt := big.NewInt(int64(i))
			benInt := big.NewInt(0)
			w.UserStates = append(w.UserStates, merkle.UserState{
				Name: nameInt,
				Ben:  benInt,
			})
		}
		// Generate the initial Merkle root
		initialRoot := merkle.BuildMerkleStates(w.UserStates)
		w.LatestRoot = merkle.MerkleRootToBase64(initialRoot)
		w.LatestRootHash = w.LatestRoot
		log.Printf("Initialized state with %d users, root: %s", maxUsers, w.LatestRoot)
	}

	oldRoot := w.LatestRootHash

	// Process each transaction in the block
	for i, tx := range transactions {
		log.Printf("Processing transaction %d: %s", i, tx.TxID)

		// Skip transaction if there are no args
		if len(tx.Args) < 1 {
			log.Printf("Transaction %s has no arguments, skipping", tx.TxID)
			continue
		}

		contractMethod := tx.Args[0]

		switch contractMethod {
		case "CurrencyContract:CreatePlayer":
			if len(tx.Args) < 2 {
				log.Printf("Invalid CreatePlayer transaction: missing arguments")
				continue
			}

			// Parse player name (usually a number in string form)
			playerName, err := strconv.ParseInt(tx.Args[1], 10, 64)
			if err != nil {
				log.Printf("Error parsing player name: %v", err)
				continue
			}
			nameInt := big.NewInt(playerName)

			// Find first dummy user (with name >= number of real users) and replace it
			found := false
			for i := range w.UserStates {
				// Check if this is a "dummy" user (name is sequential and Ben is 0)
				if w.UserStates[i].Ben.Cmp(big.NewInt(0)) == 0 &&
					w.UserStates[i].Name.Cmp(big.NewInt(nameInt.Int64())) != 0 {

					// Get old user state to generate proof
					oldState := w.UserStates[i]

					// Update user state
					w.UserStates[i].Name = nameInt

					// Generate Merkle proof for this update
					oldStateHash := merkle.HashUserState(oldState)
					proof, err := merkle.GenerateMerkleProof(w.UserStates, oldStateHash)
					if err != nil {
						log.Printf("Error generating Merkle proof: %v", err)
						continue
					}

					// Compute new Merkle root
					newRoot := merkle.UpdateMerkleRoot(proof, w.UserStates[i])
					w.LatestRootHash = merkle.MerkleRootToBase64(newRoot)

					// Save the proof
					w.StateProofs = append(w.StateProofs, *proof)
					w.StateRoots = append(w.StateRoots, w.LatestRootHash)

					found = true
					log.Printf("Created new player with name %s in slot %d", nameInt.String(), i)
					break
				}
			}

			if !found {
				log.Printf("No available slots for new player")
			}

		case "CurrencyContract:RecordBankTransaction":
			// For RecordBankTransaction, just acknowledge it
			// Since bank deposits are tracked on Layer 2, we don't need to update our states
			log.Printf("Acknowledged bank transaction: %s", tx.TxID)

		case "CurrencyContract:ExchangeInGameCurrency":
			// Check if we have enough arguments
			if len(tx.Args) < 3 { // Method, PlayerID, BenAmountChange
				log.Printf("Invalid ExchangeInGameCurrency transaction: missing arguments")
				continue
			}

			// Parse player name
			playerName, err := strconv.ParseInt(tx.Args[1], 10, 64)
			if err != nil {
				log.Printf("Error parsing player name: %v", err)
				continue
			}
			nameInt := big.NewInt(playerName)

			// Parse BEN amount change
			benAmountStr := tx.Args[2]
			benAmount, err := strconv.ParseInt(benAmountStr, 10, 64)
			if err != nil {
				log.Printf("Error parsing BEN amount: %v", err)
				continue
			}

			// Convert to a big.Int (assuming 3 decimal places precision)
			benInt := big.NewInt(benAmount)

			// Find the user to update
			found := false
			for i := range w.UserStates {
				if w.UserStates[i].Name.Cmp(nameInt) == 0 {
					// Get old state to generate proof
					oldState := w.UserStates[i]

					// Update user state (add BEN to current balance)
					newBen := new(big.Int).Add(w.UserStates[i].Ben, benInt)
					w.UserStates[i].Ben = newBen

					// Generate Merkle proof for this update
					oldStateHash := merkle.HashUserState(oldState)
					proof, err := merkle.GenerateMerkleProof(w.UserStates, oldStateHash)
					if err != nil {
						log.Printf("Error generating Merkle proof: %v", err)
						continue
					}

					// Compute new Merkle root
					newRoot := merkle.UpdateMerkleRoot(proof, w.UserStates[i])
					w.LatestRootHash = merkle.MerkleRootToBase64(newRoot)

					// Save the proof
					w.StateProofs = append(w.StateProofs, *proof)
					w.StateRoots = append(w.StateRoots, w.LatestRootHash)

					found = true
					log.Printf("Updated player %s balance by %s BEN to %s",
						nameInt.String(), benInt.String(), newBen.String())
					break
				}
			}

			if !found {
				log.Printf("Player not found for ExchangeInGameCurrency: %s", nameInt.String())
			}

		default:
			log.Printf("Unknown contract method: %s", contractMethod)
		}
	}

	// If the root has changed, update LatestRoot
	if oldRoot != w.LatestRootHash {
		w.LatestRoot = w.LatestRootHash
		log.Printf("Updated Merkle root: %s", w.LatestRoot)
	}

	return nil
}

// -------------------------------------------------------------
// Helper functions below
// -------------------------------------------------------------

// getPlayersNum evaluates a transaction to query ledger state and prints the number of players
func getPlayersNum(contract *client.Contract) {
	log.Println("\n--> Evaluate Transaction: getPlayersNum, function returns the number of current players on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("CurrencyContract:GetAllPlayers")
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	// Unmarshal the JSON bytes into a slice of player structs
	var players []*gateway.Player
	if err := json.Unmarshal(evaluateResult, &players); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}

	// Now you can accurately get the number of players
	log.Printf("*** Number of Records: %d\n", len(players))
}

// createPlayer directly create a player with all attr initialized default
func createPlayer(contract *client.Contract, playerIdStr string) {
	log.Printf("\n--> Submit Transaction: CreatePlayer \n")

	// Convert string to int64, then to big.Int
	playerId, err := strconv.ParseInt(playerIdStr, 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse player ID: %w", err))
	}

	playerIdBig := big.NewInt(playerId)

	_, err = contract.SubmitTransaction("CurrencyContract:CreatePlayer", playerIdBig.String())
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

// recordBankTransaction records a new bank transaction to the ledger
func recordBankTransaction(contract *client.Contract, userIDStr, amountUSDStr, transactionIDStr string) {
	log.Printf("\n--> Submit Transaction: RecordBankTransaction \n")

	// Convert strings to big.Int
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse user ID: %w", err))
	}

	// For USD amount, we need to convert it to the 3 decimal places format
	amountFloat, err := strconv.ParseFloat(amountUSDStr, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse USD amount: %w", err))
	}
	amountUSD := big.NewInt(int64(amountFloat * 1000)) // Multiply by 1000 for 3 decimal places

	transactionID, err := strconv.ParseInt(transactionIDStr, 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse transaction ID: %w", err))
	}

	userIDBig := big.NewInt(userID)
	transactionIDBig := big.NewInt(transactionID)

	_, err = contract.SubmitTransaction(
		"CurrencyContract:RecordBankTransaction",
		userIDBig.String(),
		amountUSD.String(),
		transactionIDBig.String(),
	)

	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}

// exchangeInGameCurrency allows users to exchange currency (USD to BEN or BEN to USD)
func exchangeInGameCurrency(contract *client.Contract, userIDStr, benAmountChangeStr string) {
	log.Printf("\n--> Submit Transaction: ExchangeInGameCurrency \n")

	// Convert strings to big.Int
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse user ID: %w", err))
	}

	// For BEN amount, we need to convert it to the 3 decimal places format
	amountFloat, err := strconv.ParseFloat(benAmountChangeStr, 64)
	if err != nil {
		panic(fmt.Errorf("failed to parse BEN amount: %w", err))
	}
	benAmountChange := big.NewInt(int64(amountFloat * 1000)) // Multiply by 1000 for 3 decimal places

	userIDBig := big.NewInt(userID)

	_, err = contract.SubmitTransaction(
		"CurrencyContract:ExchangeInGameCurrency",
		userIDBig.String(),
		benAmountChange.String(),
	)

	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
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

func commitMerkleRoot(contract *client.Contract, blockNumber, merkleRoot string) {
	log.Printf("\n--> Submit Transaction: CommitMerkleRoot \n")

	_, err := contract.SubmitTransaction("PlasmaContract:CommitMerkleRoot", blockNumber, merkleRoot)
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	log.Printf("*** Transaction committed successfully\n")
}
