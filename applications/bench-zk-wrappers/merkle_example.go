package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// -----------------------------------------------------------
// Data Structures
// -----------------------------------------------------------

// UserState represents a single user's state in the rollup
// "TxMap" stores txID -> USD
// "BEN" stores user's BEN balance.
type UserState struct {
	TxMap map[string]float64
	BEN   float64
}

// StateDB simulates our layer-2 chain state in memory.
// "Users" stores address (string) -> UserState.
type StateDB struct {
	Users map[string]*UserState
}

// NewStateDB creates a new in-memory StateDB.
func NewStateDB() *StateDB {
	return &StateDB{
		Users: make(map[string]*UserState),
	}
}

// -----------------------------------------------------------
// Constants and Helpers
// -----------------------------------------------------------

const exchangeRate = 0.313 // 1 USD = 0.313 BEN for the exchange

// getOrCreateUser returns a *UserState for the given user address.
// If the user doesn't exist in the DB, it creates a new entry.
func (db *StateDB) getOrCreateUser(user string) *UserState {
	if _, exists := db.Users[user]; !exists {
		db.Users[user] = &UserState{
			TxMap: make(map[string]float64),
			BEN:   0,
		}
	}
	return db.Users[user]
}

// BuildUserLeafString returns the "leaf" string representation for a given user
// that we store in our naive Merkle tree.
func (db *StateDB) BuildUserLeafString(userAddr string) (string, error) {
	uState, ok := db.Users[userAddr]
	if !ok {
		return "", fmt.Errorf("user '%s' not found in DB", userAddr)
	}

	// Sort the txIDs for deterministic representation
	txIDs := make([]string, 0, len(uState.TxMap))
	for tx := range uState.TxMap {
		txIDs = append(txIDs, tx)
	}
	sort.Strings(txIDs)

	// Build a string summarizing user's state
	// e.g. "bob|txID456:200.00,txID789:50.00,|250.00"
	txSummary := ""
	for _, tx := range txIDs {
		txSummary += fmt.Sprintf("%s:%.2f,", tx, uState.TxMap[tx])
	}
	leafString := fmt.Sprintf("%s|%s|%.2f", userAddr, txSummary, uState.BEN)

	return leafString, nil
}

// -----------------------------------------------------------
// Deposit Logic
// -----------------------------------------------------------

// Deposit checks if a deposit (from the bank) is valid,
// and updates the user's state if valid.
// For simplicity:
//   - The "valid" txID must start with "txID".
//   - "amount" must be positive.
func (db *StateDB) Deposit(user string, txID string, amount float64) error {
	if len(txID) < 4 || txID[:4] != "txID" {
		return fmt.Errorf("invalid txID format for deposit")
	}
	if amount <= 0 {
		return fmt.Errorf("deposit amount must be positive")
	}

	userState := db.getOrCreateUser(user)

	// If txID already exists, we can decide to reject or overwrite.
	// Here we reject for clarity.
	if _, exists := userState.TxMap[txID]; exists {
		return fmt.Errorf("txID %s already deposited", txID)
	}

	// Add the deposit to the user's TxMap
	userState.TxMap[txID] = amount
	fmt.Printf("[Deposit] User: %s, txID: %s, amount: %f (USD)\n", user, txID, amount)
	return nil
}

// -----------------------------------------------------------
// Exchange Logic
// -----------------------------------------------------------

// Exchange converts the USD from a specific txID into BEN at a fixed rate.
//   - Removes the txID from TxMap
//   - Adds (USD * exchangeRate) to user's BEN balance
//   - No partial exchange is allowed (exchange entire txID).
func (db *StateDB) Exchange(user string, txID string) error {
	userState := db.getOrCreateUser(user)

	amountUSD, exists := userState.TxMap[txID]
	if !exists {
		return fmt.Errorf("txID %s does not exist in user %s's TxMap", txID, user)
	}

	// Remove the txID from TxMap
	delete(userState.TxMap, txID)

	// Add the equivalent BEN to user's balance
	userState.BEN += amountUSD * exchangeRate
	fmt.Printf("[Exchange] User: %s, txID: %s, exchanged USD: %f, gained BEN: %f\n",
		user, txID, amountUSD, amountUSD*exchangeRate)

	return nil
}

// -----------------------------------------------------------
// Merkle Tree (Very Simplistic)
// -----------------------------------------------------------

// ComputeStateMerkleRoot builds a simple Merkle tree over all user states.
// For each user, we gather a representation of their user address + states
// and treat it as one leaf. Then we compute the Merkle root over all leaves.
// (In reality, you'd build a per-user Merkle tree of TxMap, then a global
// Merkle tree, or use more advanced data structures.)
func (db *StateDB) ComputeStateMerkleRoot() string {
	leaves := db.computeAllLeaves()
	return computeMerkleRoot(leaves)
}

// computeAllLeaves returns the sorted array of leaves (hash inputs) for all users.
func (db *StateDB) computeAllLeaves() []string {
	// Sort the users to have a deterministic order
	userAddresses := make([]string, 0, len(db.Users))
	for addr := range db.Users {
		userAddresses = append(userAddresses, addr)
	}
	sort.Strings(userAddresses)

	// Generate leaves
	var leaves []string
	for _, addr := range userAddresses {
		leafStr, _ := db.BuildUserLeafString(addr)
		leaves = append(leaves, leafStr)
	}
	return leaves
}

// computeMerkleRoot is a simple (and naive) Merkle tree computation using SHA256.
// (Pairs up leaves, hashes them until one remains).
func computeMerkleRoot(leaves []string) string {
	if len(leaves) == 0 {
		return ""
	}
	// Convert leaves to slice of hashed strings
	var level []string
	for _, leaf := range leaves {
		h := sha256.Sum256([]byte(leaf))
		level = append(level, hex.EncodeToString(h[:]))
	}

	// Build up the tree until one root is left
	for len(level) > 1 {
		var nextLevel []string
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				combined := level[i] + level[i+1]
				h := sha256.Sum256([]byte(combined))
				nextLevel = append(nextLevel, hex.EncodeToString(h[:]))
			} else {
				// Odd leaf out, carry it to next level
				nextLevel = append(nextLevel, level[i])
			}
		}
		level = nextLevel
	}
	return level[0]
}

// -----------------------------------------------------------
// Naive Merkle Proof Construction & Verification
// -----------------------------------------------------------

// BuildUserMerkleProof returns the leaf string for `userAddr`, the proof (sibling hashes),
// the pathBits (left/right indicators), and the final root. This is a "naive" approach
// that builds the entire tree in memory to extract the path.
func (db *StateDB) BuildUserMerkleProof(userAddr string) (leaf string, proof []string, pathBits []bool, root string, err error) {
	// Step 1: Get all leaves in sorted order
	leaves := db.computeAllLeaves()
	if len(leaves) == 0 {
		return "", nil, nil, "", fmt.Errorf("no leaves in the tree")
	}

	// Build leaf string for the user
	leaf, err = db.BuildUserLeafString(userAddr)
	if err != nil {
		return "", nil, nil, "", err
	}

	// Step 2: Convert each leaf to a hash
	hashedLeaves := make([]string, len(leaves))
	leafIndex := -1
	for i, l := range leaves {
		h := sha256.Sum256([]byte(l))
		hashedLeaves[i] = hex.EncodeToString(h[:])
		if l == leaf {
			leafIndex = i
		}
	}
	if leafIndex == -1 {
		return "", nil, nil, "", fmt.Errorf("user '%s' leaf not found in leaves array", userAddr)
	}

	// If there's only one leaf, it is the root. Proof is empty.
	if len(leaves) == 1 {
		return leaf, []string{}, []bool{}, hashedLeaves[0], nil
	}

	// Step 3: Build tree level-by-level, tracking the index of our leaf as it
	// goes up each level. We'll store the sibling's hash each time.
	currentLevel := hashedLeaves
	currentIndex := leafIndex
	proof = []string{}
	pathBits = []bool{} // false = left sibling, true = right sibling (or vice-versa)

	for len(currentLevel) > 1 {
		var nextLevel []string
		// We'll compute pairwise
		for i := 0; i < len(currentLevel); i += 2 {
			if i+1 < len(currentLevel) {
				combinedHash := sha256.Sum256([]byte(currentLevel[i] + currentLevel[i+1]))
				nextLevel = append(nextLevel, hex.EncodeToString(combinedHash[:]))

				// If our leaf is in this pair, record the sibling and direction
				if currentIndex == i {
					// Leaf is the left child; sibling is the right child
					proof = append(proof, currentLevel[i+1])
					pathBits = append(pathBits, false) // "I'm on the left"
					currentIndex = len(nextLevel) - 1
				} else if currentIndex == i+1 {
					// Leaf is the right child; sibling is the left child
					proof = append(proof, currentLevel[i])
					pathBits = append(pathBits, true) // "I'm on the right"
					currentIndex = len(nextLevel) - 1
				}
			} else {
				// Odd leaf out, carry up
				nextLevel = append(nextLevel, currentLevel[i])
				if currentIndex == i {
					currentIndex = len(nextLevel) - 1
				}
			}
		}
		currentLevel = nextLevel
	}

	root = currentLevel[0] // last hash standing
	return leaf, proof, pathBits, root, nil
}

// VerifyUserMerkleProof recomputes the root from the leaf + proof + pathBits
// to ensure it matches the expected root. If so, returns true.
func VerifyUserMerkleProof(leaf string, proof []string, pathBits []bool, expectedRoot string) bool {
	// Hash the leaf
	leafHash := sha256.Sum256([]byte(leaf))
	computed := hex.EncodeToString(leafHash[:])

	// Re-hash up the tree using the proof
	for i, siblingHash := range proof {
		if pathBits[i] {
			// Leaf was on the right side
			// So the order is (sibling + computed)
			h := sha256.Sum256([]byte(siblingHash + computed))
			computed = hex.EncodeToString(h[:])
		} else {
			// Leaf was on the left side
			// So the order is (computed + sibling)
			h := sha256.Sum256([]byte(computed + siblingHash))
			computed = hex.EncodeToString(h[:])
		}
	}

	// Compare final computed hash with expected root
	return (computed == expectedRoot)
}

// -----------------------------------------------------------
// Main Function (Tests)
// -----------------------------------------------------------

func main() {
	fmt.Println("=== Starting Minimal Gateway Operator Simulation ===")

	// Create a new in-memory StateDB
	db := NewStateDB()

	// 1) Test deposit
	err := db.Deposit("alice", "txID123", 100.0)
	if err != nil {
		fmt.Println("Deposit error:", err)
	}
	err = db.Deposit("bob", "txID456", 200.0)
	if err != nil {
		fmt.Println("Deposit error:", err)
	}

	// 2) Check Merkle root after deposit
	rootAfterDeposits := db.ComputeStateMerkleRoot()
	fmt.Printf("Merkle Root after deposits: %s\n", rootAfterDeposits)

	// -- DEMO: Build Merkle proof for Bob's state after deposit
	bobLeaf, bobProof, bobPathBits, bobRoot, err := db.BuildUserMerkleProof("bob")
	if err != nil {
		fmt.Println("BuildUserMerkleProof error:", err)
	} else {
		fmt.Println("\n[Bob's Merkle Proof after deposit]")
		fmt.Println("  leaf:     ", bobLeaf)
		fmt.Println("  proof:    ", bobProof)
		fmt.Println("  pathBits: ", bobPathBits)
		fmt.Println("  root:     ", bobRoot)

		// Verify
		verified := VerifyUserMerkleProof(bobLeaf, bobProof, bobPathBits, bobRoot)
		fmt.Printf("  Verified? %v\n", verified)
	}

	// 3) Test exchange (Alice)
	err = db.Exchange("alice", "txID123")
	if err != nil {
		fmt.Println("Exchange error:", err)
	}

	// Another deposit for Bob
	err = db.Deposit("bob", "txID789", 50.0)
	if err != nil {
		fmt.Println("Deposit error:", err)
	}

	// 4) Check Merkle root after exchange
	rootAfterExchange := db.ComputeStateMerkleRoot()
	fmt.Printf("\nMerkle Root after exchange: %s\n", rootAfterExchange)

	// -- DEMO: Build and verify a new Merkle proof for Bob
	bobLeaf2, bobProof2, bobPathBits2, bobRoot2, err := db.BuildUserMerkleProof("bob")
	if err != nil {
		fmt.Println("BuildUserMerkleProof error:", err)
	} else {
		fmt.Println("\n[Bob's Merkle Proof after exchange+deposit]")
		fmt.Println("  leaf:     ", bobLeaf2)
		fmt.Println("  proof:    ", bobProof2)
		fmt.Println("  pathBits: ", bobPathBits2)
		fmt.Println("  root:     ", bobRoot2)

		verified := VerifyUserMerkleProof(bobLeaf2, bobProof2, bobPathBits2, bobRoot2)
		fmt.Printf("  Verified? %v\n", verified)
	}

	// Print final states
	fmt.Println("\n[Final States]")
	for user, state := range db.Users {
		fmt.Printf("User: %s, TxMap: %+v, BEN: %.2f\n", user, state.TxMap, state.BEN)
	}

	fmt.Println("=== End of Simulation ===")
}
