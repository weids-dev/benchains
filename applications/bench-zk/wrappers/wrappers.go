// wrappers/wrappers.go
package wrappers

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"time"

	"bench-zk/gateway"
	"bench-zk/merkle"

	"github.com/hyperledger/fabric-gateway/pkg/client"
)

// UserState holds a user's name, balance ($BEN).
// Each UserState will be commit to the transaction root every period of time.
// When exiting, user can withdraw all BEN stored on zk-rollups contract chain,
// but he/she cannot move any unused deposits out.
type UserState struct {
	Name string
	BEN  *big.Int
}

// Deposit represents an unused deposits which root will also being commited to mainchain.
type Deposit struct {
	TxID string
	Name string
	USD  *big.Int
}

// The Operator wiil use UserState root as input to generate proof for exchangeBen
// The Operator will use Deposit root as input to generate proof for depositTransaction
type Wrappers struct {
	UserStates   []UserState
	Deposits     []Deposit
	Transactions []merkle.TransactionData
	StateRoots   []string         // set of intermediate states between each transactions
	StateProofs  []MProof         // each Merkle proof to show that the state is exactly in the tree root
	Gw1          *gateway.Gateway // Gw1 represents the way operator communicate with Layer 1
	Gw2          *gateway.Gateway // Gw2 represents the way operator communicate with Layer 2
}

// MProof represents a Merkle proof
type MProof struct {
	// Add necessary fields for your Merkle proof
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

	// Initialize Wrappers with empty UserStates and Deposits
	return &Wrappers{
		UserStates:   []UserState{},
		Deposits:     []Deposit{},
		Transactions: []merkle.TransactionData{},
		StateRoots:   []string{}, // set of intermediate states between each transactions
		StateProofs:  []MProof{}, // each Merkle proof to show that the state is exactly in the tree root.
		Gw1:          gw1,
		Gw2:          gw2,
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

func (w *Wrappers) Operate() error {
	newestCommittedBlockNumber := uint64(1) // Initially, no blocks committed
	// Periodic loop to check the newest block and fetch new blocks
	ticker := time.NewTicker(5 * time.Second) // 5 seconds interval
	defer ticker.Stop()

	for range ticker.C {
		// Step 1: Get the newest block number
		syscontract := w.Gw2.Gateway.GetNetwork(w.Gw2.ChannelName).GetContract("qscc")
		newestBlockNumber, err := getNewestBlockNumber(syscontract, w.Gw2.ChannelName)

		if err != nil {
			log.Println("Error getting newest block number:", err)
			continue
		}

		fmt.Printf("Newest block number: %d", newestBlockNumber)
		fmt.Printf(" || Newest committed block number: %d \n", newestCommittedBlockNumber)

		// Step 2: Check if there are new blocks to process
		if newestBlockNumber > newestCommittedBlockNumber {
			fmt.Printf("Found new blocks to commit: %d to %d    || ", newestCommittedBlockNumber+1, newestBlockNumber)

			// Step 3: Process all blocks between newestCommittedBlockNumber + 1 and newestBlockNumber
			for blockNumber := newestCommittedBlockNumber + 1; blockNumber <= newestBlockNumber; blockNumber++ {
				snum := strconv.FormatUint(blockNumber, 10)
				blockBytes := getBlockByNumber(syscontract, w.Gw2.ChannelName, snum)
				block, err := decodeBlock(blockBytes) // decode the actual block contents
				if err != nil {
					fmt.Println("Error decoding block:", err)
					continue
				}

				w.Transactions, err = extractTransactions(block)
				if err != nil {
					fmt.Println("Error extracting transactions:", err)
					continue
				}

				// Output the extracted transactions
				for _, tx := range w.Transactions {
					log.Printf("TxID: %s\n", tx.TxID)
					for _, arg := range tx.Args {
						log.Printf("Arg: %+v\n", arg)
					}
				}
				fmt.Printf("Number of transactions in this block: %d   || ", len(w.Transactions))

				// Compute the Merkle root of the transactions
				transactionRoot := merkle.BuildMerkleTransactions(w.Transactions)
				merkleRoot := merkle.MerkleRootToBase64(transactionRoot)

				// Commit the Merkle root to the root chain
				go commitMerkleRoot(w.Gw1.Contract, snum, merkleRoot)
				fmt.Printf("Committed Merkle root for block %d: %s\n", blockNumber, merkleRoot)

				// So-far, plasma, the following part, zk-rollup
				// Step 4: Update newest committed block number after processing
				newestCommittedBlockNumber = newestBlockNumber
			}
		}
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
