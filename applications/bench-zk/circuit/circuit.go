// circuit/circuit.go

package circuit

// Implementing ZK-SNARKs Circuit using gnark library for ZK-Rollups

import (
	// "fmt"
	// "math/big"

	// ---------------------------
	//  GNARK libraries
	// ---------------------------
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"
	// ---------------------------
	//  GNARK-CRYPTO libraries
	// ---------------------------
)

// Adjustable constants for ProofMerkleCircuit
const (
	MD = 4 // Merkle Circuit

	D = 12   // Depth of the Merkle tree
	N = 4096 // Number of leaves
	B = 64   // Batch size

	D2 = 12 // ProofMerkleCircuit: Number of Leaves would be 2^15 = 32768
	B2 = 64 // Number of transactions in the batch
)

// DepositCircuit enforces that:
//
//  1. BobNewBalance = BobOldBalance + DepositAmount
//  2. OldRoot  = MiMC(BobOldBalance,  SiblingBalance)
//  3. NewRoot  = MiMC(BobNewBalance,  SiblingBalance)
//
// We consider only 2 leaves (Bob and Alice) => 1 parent node = Merkle root.
type DepositCircuit struct {
	// Public inputs
	OldRoot       frontend.Variable `gnark:"oldRoot,public"`
	NewRoot       frontend.Variable `gnark:"newRoot,public"`
	DepositAmount frontend.Variable `gnark:"depositAmount,public"`

	// Private inputs
	BobOldBalance  frontend.Variable `gnark:"bobOldBalance"`
	BobNewBalance  frontend.Variable `gnark:"bobNewBalance"`
	SiblingBalance frontend.Variable `gnark:"siblingBalance"` // Alice's balance, in this simplified example
}

// UserStateCircuit verifies the update of a user's state in a two-user Merkle tree.
// It ensures that the old and new Merkle roots are correctly computed based on the
// user’s state transition and the unchanged sibling’s hash.
type UserStateCircuit struct {
	// Public inputs
	OldRoot frontend.Variable `gnark:"oldRoot,public"`
	NewRoot frontend.Variable `gnark:"newRoot,public"`

	// Private inputs
	OldName     frontend.Variable `gnark:"oldName"`
	OldBalance  frontend.Variable `gnark:"oldBalance"`
	NewName     frontend.Variable `gnark:"newName"`
	NewBalance  frontend.Variable `gnark:"newBalance"`
	SiblingHash frontend.Variable `gnark:"siblingHash"`
	PathBit     frontend.Variable `gnark:"pathBit"` // 0 (right) or 1 (left)
}

// MerkleCircuit verifies a state update for a single user in a Merkle tree.
type MerkleCircuit struct {
	// Public inputs
	OldRoot frontend.Variable `gnark:"oldRoot,public"`
	NewRoot frontend.Variable `gnark:"newRoot,public"`

	// Private inputs
	OldName    frontend.Variable     `gnark:"oldName"`
	OldBalance frontend.Variable     `gnark:"oldBalance"`
	NewName    frontend.Variable     `gnark:"newName"`
	NewBalance frontend.Variable     `gnark:"newBalance"`
	Siblings   [MD]frontend.Variable `gnark:"siblings"`
	PathBits   [MD]frontend.Variable `gnark:"pathBits"`
}

// BatchMerkleCircuit verifies a batch of up to 32 transactions updating a Merkle tree.
type BatchMerkleCircuit struct {
	// Public inputs
	OldRoot      frontend.Variable `gnark:"oldRoot,public"`
	NewRoot      frontend.Variable `gnark:"newRoot,public"`
	Transactions [B]struct {
		LeafIndex     frontend.Variable `gnark:"leafIndex"`
		DepositAmount frontend.Variable `gnark:"depositAmount"`
	} `gnark:",public"`

	// Private inputs
	InitialLeaves [N]struct {
		Name frontend.Variable `gnark:"name"`
		Ben  frontend.Variable `gnark:"ben"`
	}
}

// computeMerkleRoot computes the Merkle root from an array of leaves.
// Time complexity: O(N) where N is the number of leaves.
// In our zk-SNARK circuit, the time is dominated by the MiMC hash function in the function.
func computeMerkleRoot(api frontend.API, leaves [N]frontend.Variable) frontend.Variable {
	// Level 0: Leaf hashes (already provided)
	currentLevel := leaves[:]

	// Iterate through D=4 levels to compute the root
	for level := 0; level < D; level++ {
		nextLevelSize := len(currentLevel) / 2
		nextLevel := make([]frontend.Variable, nextLevelSize)
		for i := 0; i < nextLevelSize; i++ {
			mimcHash, _ := mimc.NewMiMC(api)
			mimcHash.Write(currentLevel[2*i], currentLevel[2*i+1])
			nextLevel[i] = mimcHash.Sum()
		}
		currentLevel = nextLevel
	}
	return currentLevel[0] // Root
}

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
		NewBalance frontend.Variable     `gnark:"newBalance"`
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

		// Compute new leaf hash: H_new_k = MiMC(NewName, NewBalance)
		mimcNew, err := mimc.NewMiMC(api)
		if err != nil {
			return err
		}
		mimcNew.Write(tx.NewName, tx.NewBalance)
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

// Define implements the circuit constraints.
func (c *BatchMerkleCircuit) Define(api frontend.API) error {
	// Step 1: Compute initial leaf hashes and Merkle root
	/*
		var initialHashes [N]frontend.Variable
		for i := 0; i < N; i++ {
			mimcLeaf, err := mimc.NewMiMC(api)
			if err != nil {
				return err
			}
			mimcLeaf.Write(c.InitialLeaves[i].Name, c.InitialLeaves[i].Ben)
			initialHashes[i] = mimcLeaf.Sum()
		}
		initialRoot := computeMerkleRoot(api, initialHashes)
		api.AssertIsEqual(initialRoot, c.OldRoot)
	*/

	// Step 2: Compute total deposit per leaf
	var totalDeposits [N]frontend.Variable
	for i := 0; i < N; i++ {
		totalDeposits[i] = 0 // Initialize to zero directly
		for k := 0; k < B; k++ {
			// Check if transaction k targets leaf i
			// Compute delta = LeafIndex[k] - i
			delta := api.Sub(c.Transactions[k].LeafIndex, i)
			// isTarget = 1 if delta == 0 (i.e., LeafIndex[k] == i), else 0
			isTarget := api.IsZero(delta)
			// deposit = isTarget * DepositAmount[k] (if isTarget is 0, deposit is 0)
			deposit := api.Mul(isTarget, c.Transactions[k].DepositAmount)
			// Accumulate deposit
			totalDeposits[i] = api.Add(totalDeposits[i], deposit)
		}
	}

	// Step 3: Compute final leaf states
	var finalBen [N]frontend.Variable
	for i := 0; i < N; i++ {
		finalBen[i] = api.Add(c.InitialLeaves[i].Ben, totalDeposits[i])
	}

	// Step 4: Compute final leaf hashes and Merkle root
	var finalHashes [N]frontend.Variable
	for i := 0; i < N; i++ {
		mimcLeaf, err := mimc.NewMiMC(api)
		if err != nil {
			return err
		}
		// Name remains unchanged
		mimcLeaf.Write(c.InitialLeaves[i].Name, finalBen[i])
		finalHashes[i] = mimcLeaf.Sum()
	}
	finalRoot := computeMerkleRoot(api, finalHashes)
	api.AssertIsEqual(finalRoot, c.NewRoot)

	// Step 5: Ensure LeafIndex values are valid (0 to N-1)
	/*
		for k := 0; k < B; k++ {
			// Assert 0 <= LeafIndex[k] <= N-1
			api.AssertIsLessOrEqual(0, c.Transactions[k].LeafIndex)
			api.AssertIsLessOrEqual(c.Transactions[k].LeafIndex, N-1)
		}
	*/

	return nil
}

// Define implements the circuit constraints.
func (c *MerkleCircuit) Define(api frontend.API) error {
	// Compute old leaf hash: H_old = MiMC(OldName, OldBalance)
	mimcOld, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	mimcOld.Write(c.OldName, c.OldBalance)
	H_old := mimcOld.Sum()

	// Compute old root from H_old and Merkle proof
	currentHashOld := H_old
	for i := 0; i < MD; i++ {
		// If PathBits[i] = 1 (true), current node is left: hash(current, sibling)
		// If PathBits[i] = 0 (false), current node is right: hash(sibling, current)
		left := api.Select(c.PathBits[i], currentHashOld, c.Siblings[i])
		right := api.Select(c.PathBits[i], c.Siblings[i], currentHashOld)
		mimcLevel, err := mimc.NewMiMC(api)
		if err != nil {
			return err
		}
		mimcLevel.Write(left, right)
		currentHashOld = mimcLevel.Sum()
	}
	api.AssertIsEqual(currentHashOld, c.OldRoot)

	// Compute new leaf hash: H_new = MiMC(NewName, NewBalance)
	mimcNew, err := mimc.NewMiMC(api)
	if err != nil {
		return err
	}
	mimcNew.Write(c.NewName, c.NewBalance)
	H_new := mimcNew.Sum()

	// Compute new root from H_new and the same Merkle proof
	currentHashNew := H_new
	for i := 0; i < MD; i++ {
		left := api.Select(c.PathBits[i], currentHashNew, c.Siblings[i])
		right := api.Select(c.PathBits[i], c.Siblings[i], currentHashNew)
		mimcLevel, err := mimc.NewMiMC(api)
		if err != nil {
			return err
		}
		mimcLevel.Write(left, right)
		currentHashNew = mimcLevel.Sum()
	}
	api.AssertIsEqual(currentHashNew, c.NewRoot)

	// Constrain PathBits to be boolean (0 or 1)
	for i := 0; i < MD; i++ {
		api.AssertIsBoolean(c.PathBits[i])
	}

	return nil
}

// Define implements the circuit constraints.
func (c *UserStateCircuit) Define(api frontend.API) error {
	// Compute hash of the old user state: H_old = MiMC(OldName, OldBalance)
	mimcOld, _ := mimc.NewMiMC(api)
	mimcOld.Write(c.OldName, c.OldBalance)
	H_old := mimcOld.Sum()

	// Compute hash of the new user state: H_new = MiMC(NewName, NewBalance)
	mimcNew, _ := mimc.NewMiMC(api)
	mimcNew.Write(c.NewName, c.NewBalance)
	H_new := mimcNew.Sum()

	// Compute the old root based on PathBit
	leftOld := api.Select(c.PathBit, H_old, c.SiblingHash)  // PathBit=1: H_old, PathBit=0: SiblingHash
	rightOld := api.Select(c.PathBit, c.SiblingHash, H_old) // PathBit=1: SiblingHash, PathBit=0: H_old
	mimcRootOld, _ := mimc.NewMiMC(api)
	mimcRootOld.Write(leftOld, rightOld)
	computedOldRoot := mimcRootOld.Sum()

	// Compute the new root based on PathBit
	leftNew := api.Select(c.PathBit, H_new, c.SiblingHash)  // PathBit=1: H_new, PathBit=0: SiblingHash
	rightNew := api.Select(c.PathBit, c.SiblingHash, H_new) // PathBit=1: SiblingHash, PathBit=0: H_new
	mimcRootNew, _ := mimc.NewMiMC(api)
	mimcRootNew.Write(leftNew, rightNew)
	computedNewRoot := mimcRootNew.Sum()

	// Enforce that computed roots match the public inputs
	api.AssertIsEqual(computedOldRoot, c.OldRoot)
	api.AssertIsEqual(computedNewRoot, c.NewRoot)

	return nil
}

// Define implements the circuit constraints.
func (c *DepositCircuit) Define(api frontend.API) error {
	// 1) Enforce new balance = old balance + deposit
	bobComputedNew := api.Add(c.BobOldBalance, c.DepositAmount)
	api.AssertIsEqual(bobComputedNew, c.BobNewBalance)

	// 2) Recompute old root = MiMC(BobOldBalance, SiblingBalance)
	mimcOld, _ := mimc.NewMiMC(api) // in-circuit MiMC
	mimcOld.Write(c.BobOldBalance, c.SiblingBalance)
	computedOldRoot := mimcOld.Sum()
	api.AssertIsEqual(computedOldRoot, c.OldRoot)

	// 3) Recompute new root = MiMC(BobNewBalance, SiblingBalance)
	mimcNew, _ := mimc.NewMiMC(api)
	mimcNew.Write(c.BobNewBalance, c.SiblingBalance)
	computedNewRoot := mimcNew.Sum()
	api.AssertIsEqual(computedNewRoot, c.NewRoot)

	// fmt.Print(c.NewRoot)
	// fmt.Println()
	// fmt.Print(computedNewRoot)

	return nil
}
