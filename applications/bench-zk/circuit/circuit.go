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

// D is the depth of the Merkle tree (4 levels for 16 users)
const D = 4

// DepositCircuit enforces that:
//
//   1) BobNewBalance = BobOldBalance + DepositAmount
//   2) OldRoot  = MiMC(BobOldBalance,  SiblingBalance)
//   3) NewRoot  = MiMC(BobNewBalance,  SiblingBalance)
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
	OldName    frontend.Variable    `gnark:"oldName"`
	OldBalance frontend.Variable    `gnark:"oldBalance"`
	NewName    frontend.Variable    `gnark:"newName"`
	NewBalance frontend.Variable    `gnark:"newBalance"`
	Siblings   [D]frontend.Variable `gnark:"siblings"`
	PathBits   [D]frontend.Variable `gnark:"pathBits"`
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
	for i := 0; i < D; i++ {
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
	for i := 0; i < D; i++ {
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
	for i := 0; i < D; i++ {
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
