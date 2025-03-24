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

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/hyperledger/fabric-gateway/pkg/client"
)

// The Operator wiil use UserState root as input to generate proof for exchangeBen
// The Operator will use Deposit root as input to generate proof for depositTransaction
type Wrappers struct {
	UserStates        []merkle.UserState
	StateRoots        []string                 // set of intermediate states between each transactions
	StateProofs       []merkle.MProof          // each Merkle proof to show that the state is exactly in the tree root
	Gw1               *gateway.Gateway         // Gw1 represents the way operator communicate with Layer 1
	Gw2               *gateway.Gateway         // Gw2 represents the way operator communicate with Layer 2
	LatestRoot        int64                    // The latest root committed to Layer 1
	LatestRootHash    string                   // The latest root hash committed to Layer 1
	BlockTransactions []merkle.TransactionData // Store transactions for current block
	DummyUserIndex    int                      // Index of the next available dummy user slot

	// ZK circuit related fields
	ProofCircuit        *circuit.ProofMerkleCircuit // The circuit for generating proofs
	CircuitR1CS         constraint.ConstraintSystem // Compiled circuit
	ProvingKey          interface{}                 // Proving key for the circuit
	VerifyingKey        interface{}                 // Verifying key for the circuit
	Initialized         bool                        // Flag to track if circuit is initialized
	CircuitTransactions []struct {                  // Pre-prepared transaction data for the circuit
		OldName    *big.Int
		OldBalance *big.Int
		NewName    *big.Int
		BenChange  *big.Int
		Siblings   []*big.Int
		PathBits   []bool
	}
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

	// Initialize ZK Circuit
	log.Println("Initializing ZK circuit...")
	var zkCircuit circuit.ProofMerkleCircuit

	// Compile the circuit to R1CS
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &zkCircuit)
	if err != nil {
		gw1.Close()
		gw2.Close()
		return nil, fmt.Errorf("failed to compile ZK circuit: %w", err)
	}

	// Setup proving and verifying keys
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		gw1.Close()
		gw2.Close()
		return nil, fmt.Errorf("failed to setup ZK proving/verifying keys: %w", err)
	}
	log.Println("ZK circuit initialized successfully")

	// Initialize Wrappers with empty UserStates and Deposits
	return &Wrappers{
		UserStates:        []merkle.UserState{},
		StateRoots:        []string{},        // set of intermediate states between each transactions
		StateProofs:       []merkle.MProof{}, // each Merkle proof to show that the state is exactly in the tree root.
		Gw1:               gw1,
		Gw2:               gw2,
		LatestRoot:        0,
		LatestRootHash:    "",
		BlockTransactions: []merkle.TransactionData{},
		DummyUserIndex:    0,
		ProofCircuit:      &zkCircuit,
		CircuitR1CS:       ccs,
		ProvingKey:        pk,
		VerifyingKey:      vk,
		Initialized:       true,
		CircuitTransactions: []struct {
			OldName    *big.Int
			OldBalance *big.Int
			NewName    *big.Int
			BenChange  *big.Int
			Siblings   []*big.Int
			PathBits   []bool
		}{},
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

					transactions, err := extractTransactions(block)
					if err != nil {
						fmt.Println("Error extracting transactions:", err)
						continue
					}

					fmt.Printf("Number of transactions in this block: %d   || ", len(transactions))

					// Clear block transactions before processing new ones
					w.BlockTransactions = []merkle.TransactionData{}

					// Process transactions before computing Merkle root
					err = w.processTransactions(transactions)
					if err != nil {
						fmt.Printf("Error processing transactions: %v\n", err)
						continue
					}
					// TODO: If there is no transaction that will change the UserStates, skip the proof generation

					// Generate ZK proof for this block
					oldRoot, newRoot, proof, err := w.generateZKProof()
					if err != nil {
						fmt.Printf("Error generating ZK proof: %v\n", err)
						continue
					}

					log.Printf("Old root: %v, New root: %v", oldRoot, newRoot)
					log.Printf("Proof: %v", proof)

					// Commit the latest Merkle root with its proof to Layer 1

					// go w.commitRootWithProof(snum, w.LatestRootHash, proof, oldRoot, newRoot)
					fmt.Printf("Committed Merkle root for block %d: %s with ZK proof\n", blockNumber, w.LatestRootHash)
				}
				newestCommittedBlockNumber = newestBlockNumber
			}
		}
	}
}

// generateZKProof generates a ZK proof for the current block's transactions
func (w *Wrappers) generateZKProof() (*big.Int, *big.Int, []byte, error) {
	if !w.Initialized {
		return nil, nil, nil, fmt.Errorf("ZK circuit not initialized")
	}

	if len(w.StateRoots) < 2 {
		log.Printf("Possible reason: No transactions that will change the UserStates in this block")
		return nil, nil, nil, nil
	}

	oldRootBase64 := w.StateRoots[0]
	newRootBase64 := w.StateRoots[len(w.StateRoots)-1]
	log.Printf("Old root: %v, New root: %v", oldRootBase64, newRootBase64)

	oldRootBytes, err := merkle.Base64ToBytes(oldRootBase64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode old root: %w", err)
	}
	oldRoot := new(big.Int).SetBytes(oldRootBytes)

	newRootBytes, err := merkle.Base64ToBytes(newRootBase64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode new root: %w", err)
	}
	newRoot := new(big.Int).SetBytes(newRootBytes)

	var assignment circuit.ProofMerkleCircuit
	assignment.OldRoot = oldRoot
	assignment.NewRoot = newRoot

	txCount := len(w.CircuitTransactions)
	if txCount > circuit.B2 {
		txCount = circuit.B2
	}
	log.Printf("Generating ZK proof for %d transactions", txCount)

	// Process real transactions
	for k := 0; k < txCount; k++ {
		ctxData := w.CircuitTransactions[k]
		var pathBits [circuit.D2]frontend.Variable
		for i := 0; i < circuit.D2; i++ {
			if i < len(ctxData.PathBits) {
				if ctxData.PathBits[i] {
					pathBits[i] = big.NewInt(1)
				} else {
					pathBits[i] = big.NewInt(0)
				}
			} else {
				pathBits[i] = big.NewInt(0)
			}
		}
		var siblings [circuit.D2]frontend.Variable
		for i := 0; i < circuit.D2; i++ {
			if i < len(ctxData.Siblings) {
				siblings[i] = ctxData.Siblings[i]
			} else {
				siblings[i] = big.NewInt(0)
			}
		}
		assignment.Transactions[k].OldName = ctxData.OldName
		assignment.Transactions[k].OldBalance = ctxData.OldBalance
		assignment.Transactions[k].NewName = ctxData.NewName
		assignment.Transactions[k].BenChange = ctxData.BenChange
		assignment.Transactions[k].Siblings = siblings
		assignment.Transactions[k].PathBits = pathBits
	}

	// Fill remaining slots with valid dummy transactions
	for k := txCount; k < circuit.B2; k++ {
		// Use leaf index 0 from current state as a dummy
		dummyIndex := 0
		oldState := w.UserStates[dummyIndex]
		oldStateHash := merkle.HashUserState(oldState)
		proof, err := merkle.GenerateMerkleProof(w.UserStates, oldStateHash)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to generate dummy proof: %w", err)
		}
		var pathBits [circuit.D2]frontend.Variable
		for i := 0; i < circuit.D2; i++ {
			if i < len(proof.PathBits) {
				if proof.PathBits[i] {
					pathBits[i] = big.NewInt(1)
				} else {
					pathBits[i] = big.NewInt(0)
				}
			} else {
				pathBits[i] = big.NewInt(0)
			}
		}
		var siblings [circuit.D2]frontend.Variable
		for i := 0; i < circuit.D2; i++ {
			if i < len(proof.Siblings) {
				siblings[i] = proof.Siblings[i]
			} else {
				siblings[i] = big.NewInt(0)
			}
		}
		assignment.Transactions[k].OldName = oldState.Name
		assignment.Transactions[k].OldBalance = oldState.Ben
		assignment.Transactions[k].NewName = oldState.Name
		assignment.Transactions[k].BenChange = big.NewInt(0)
		assignment.Transactions[k].Siblings = siblings
		assignment.Transactions[k].PathBits = pathBits
	}

	log.Println("Creating witness for ZK proof...")
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create witness: %w", err)
	}

	log.Println("Generating ZK proof...")
	start := time.Now()
	proof, err := groth16.Prove(w.CircuitR1CS, w.ProvingKey.(groth16.ProvingKey), fullWitness)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate proof: %w", err)
	}
	log.Printf("ZK proof generated in %v", time.Since(start))

	proofBytes, err := serializeProof(proof)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to serialize proof: %w", err)
	}

	w.StateProofs = []merkle.MProof{}
	w.StateRoots = []string{newRootBase64}
	w.CircuitTransactions = []struct {
		OldName    *big.Int
		OldBalance *big.Int
		NewName    *big.Int
		BenChange  *big.Int
		Siblings   []*big.Int
		PathBits   []bool
	}{}

	return oldRoot, newRoot, proofBytes, nil
}

func (w *Wrappers) processTransactions(transactions []merkle.TransactionData) error {
	if len(transactions) == 0 {
		return nil
	}

	// Store transactions for this block
	w.BlockTransactions = transactions

	// If this is the first transaction block, initialize our state
	if len(w.UserStates) == 0 {
		// First try to get existing players from the blockchain
		log.Println("Initializing user states from blockchain...")

		// Call GetAllPlayers using the correct chaincode name from configuration
		currencyContract := w.Gw2.Gateway.GetNetwork(w.Gw2.ChannelName).GetContract(w.Gw2.ChaincodeName)
		evaluateResult, err := currencyContract.EvaluateTransaction("CurrencyContract:GetAllPlayers")
		if err != nil {
			log.Printf("Error getting players from blockchain: %v", err)
			// Continue with empty array if we can't get players
		}

		// Unmarshal the JSON bytes into a slice of player structs
		var players []*gateway.Player
		if len(evaluateResult) > 0 {
			if err := json.Unmarshal(evaluateResult, &players); err != nil {
				log.Printf("Failed to unmarshal players: %v", err)
				// Try direct string conversion as a fallback
				playersStr := string(evaluateResult)
				if err := json.Unmarshal([]byte(playersStr), &players); err != nil {
					log.Printf("Failed again to unmarshal players: %v", err)
				}
			}
		}

		// Initialize user states with existing players first
		log.Printf("Found %d existing players in blockchain", len(players))
		existingPlayerCount := len(players)

		// Add existing players to UserStates
		for _, player := range players {
			// Convert float64 balance to int64 (assuming 3 decimal places)
			balanceInt := int64(player.Balance * 1000)

			nameInt := big.NewInt(player.ID)
			benInt := big.NewInt(balanceInt)
			log.Printf("Find Existing Player ID: %d, Balance: %d", player.ID, balanceInt)
			w.UserStates = append(w.UserStates, merkle.UserState{
				Name: nameInt,
				Ben:  benInt,
			})
		}

		// Fill remaining slots with dummy users
		maxUsers := 1 << circuit.D2 // 2^10 = 1024 users
		for i := existingPlayerCount; i < maxUsers; i++ {
			nameInt := big.NewInt(int64(i + 1)) // Names start at 1
			benInt := big.NewInt(0)
			w.UserStates = append(w.UserStates, merkle.UserState{
				Name: nameInt,
				Ben:  benInt,
			})
		}

		// Set the DummyUserIndex to the first dummy user
		w.DummyUserIndex = existingPlayerCount
		log.Printf("DummyUserIndex: %d", w.DummyUserIndex)

		// Generate the initial Merkle root
		initialRoot := merkle.BuildMerkleStates(w.UserStates)
		// LatestRoot is the block number of the initial root
		w.LatestRoot = 0
		w.LatestRootHash = merkle.MerkleRootToBase64(initialRoot)

		// Store the initial root in StateRoots
		w.StateRoots = append(w.StateRoots, w.LatestRootHash)

		log.Printf("Initialized state with %d users (%d existing, %d dummy), root: %s",
			maxUsers, existingPlayerCount, maxUsers-existingPlayerCount, w.LatestRootHash)
	}

	// Clear CircuitTransactions before processing new transactions
	w.CircuitTransactions = []struct {
		OldName    *big.Int
		OldBalance *big.Int
		NewName    *big.Int
		BenChange  *big.Int
		Siblings   []*big.Int
		PathBits   []bool
	}{}

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
			log.Printf("CreatePlayer transaction args: %v", tx.Args)
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

			// Check if dummy user slots are available
			if w.DummyUserIndex >= len(w.UserStates) {
				log.Printf("No available slots for new player")
				continue
			}

			dummyIndex := w.DummyUserIndex
			if dummyIndex >= len(w.UserStates) {
				log.Printf("No available slots for new player")
				continue
			}

			// Get old state and generate proof *before* update
			oldState := w.UserStates[dummyIndex]
			oldStateHash := merkle.HashUserState(oldState)
			proof, err := merkle.GenerateMerkleProof(w.UserStates, oldStateHash)
			if err != nil {
				log.Printf("Error generating Merkle proof: %v", err)
				continue
			}

			// Now update the state
			w.UserStates[dummyIndex].Name = nameInt // Ben remains 0
			w.DummyUserIndex++

			// Compute new root
			newRoot := merkle.UpdateMerkleRoot(proof, w.UserStates[dummyIndex])
			w.LatestRootHash = merkle.MerkleRootToBase64(newRoot)

			// Store proof and root
			w.StateProofs = append(w.StateProofs, *proof)
			w.StateRoots = append(w.StateRoots, w.LatestRootHash)

			// Prepare circuit transaction
			w.CircuitTransactions = append(w.CircuitTransactions, struct {
				OldName    *big.Int
				OldBalance *big.Int
				NewName    *big.Int
				BenChange  *big.Int
				Siblings   []*big.Int
				PathBits   []bool
			}{
				OldName:    oldState.Name,
				OldBalance: oldState.Ben,
				NewName:    nameInt,
				BenChange:  big.NewInt(0),
				Siblings:   proof.Siblings,
				PathBits:   proof.PathBits,
			})

			log.Printf("Created new player with name %s in slot %d", nameInt.String(), dummyIndex)

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

			found := false
			for i := range w.UserStates {
				if w.UserStates[i].Name.Cmp(nameInt) == 0 {
					// Get old state and generate proof *before* update
					oldState := w.UserStates[i]
					oldStateHash := merkle.HashUserState(oldState)
					proof, err := merkle.GenerateMerkleProof(w.UserStates, oldStateHash)
					if err != nil {
						log.Printf("Error generating Merkle proof: %v", err)
						continue
					}

					// Now update the state
					newBen := new(big.Int).Add(oldState.Ben, benInt)
					w.UserStates[i].Ben = newBen

					// Compute new root
					newRoot := merkle.UpdateMerkleRoot(proof, w.UserStates[i])
					w.LatestRootHash = merkle.MerkleRootToBase64(newRoot)

					// Store proof and root
					w.StateProofs = append(w.StateProofs, *proof)
					w.StateRoots = append(w.StateRoots, w.LatestRootHash)

					// Prepare circuit transaction
					w.CircuitTransactions = append(w.CircuitTransactions, struct {
						OldName    *big.Int
						OldBalance *big.Int
						NewName    *big.Int
						BenChange  *big.Int
						Siblings   []*big.Int
						PathBits   []bool
					}{
						OldName:    oldState.Name,
						OldBalance: oldState.Ben,
						NewName:    nameInt,
						BenChange:  benInt,
						Siblings:   proof.Siblings,
						PathBits:   proof.PathBits,
					})

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
			// No need to throw error here, just log it
			log.Printf("Unknown contract method: %s", contractMethod)
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

// serializeProof converts a groth16.Proof to a byte slice
func serializeProof(proof groth16.Proof) ([]byte, error) {
	// Use the json package for serialization
	proofBytes, err := json.Marshal(proof)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal proof to JSON: %w", err)
	}
	return proofBytes, nil
}

// deserializeProof converts a byte slice back to a groth16.Proof
func deserializeProof(proofBytes []byte) (groth16.Proof, error) {
	var proof groth16.Proof
	err := json.Unmarshal(proofBytes, &proof)
	if err != nil {
		return proof, fmt.Errorf("failed to unmarshal proof from JSON: %w", err)
	}
	return proof, nil
}
