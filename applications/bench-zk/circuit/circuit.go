// circuit/circuit.go

package circuit

// Implementing ZK-SNARKs Circuit using gnark library for ZK-Rollups

import (
	// "fmt"
	"math/big"

	// ---------------------------
	//  GNARK libraries
	// ---------------------------
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash/mimc"

	// ---------------------------
	//  GNARK-CRYPTO libraries
	// ---------------------------
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	gcHash "github.com/consensys/gnark-crypto/hash"
)

// Constants for the circuit
const (
	NameLength = 10  // Maximum name length, padded with zeros
	TreeDepth  = 5   // Depth for up to 32 leaves (log2(24) ≈ 5)
)


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


// MerkleUpdateCircuit verifies a Merkle root update for a single leaf change.
type MerkleUpdateCircuit struct {
	// Public inputs
	OldRoot       frontend.Variable `gnark:"oldRoot,public"`
	NewRoot       frontend.Variable `gnark:"newRoot,public"`
	DepositAmount frontend.Variable `gnark:"depositAmount,public"`

    // Private inputs
    OldUserState  UserStateCircuit
    NewUserState  UserStateCircuit
    PathBits      [TreeDepth]frontend.Variable // Merkle path directions (0 or 1)
    Siblings      [TreeDepth]frontend.Variable // Sibling hashes along the path
}

// UserStateCircuit represents a user state in the circuit
type UserStateCircuit struct {
    NameBytes    [NameLength]frontend.Variable // Name as byte array
    Balance      frontend.Variable             // Balance as field element
    BalanceBytes [32]frontend.Variable         // Balance as 32-byte array for hashing
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

// Define implements the circuit constraints
func (c *MerkleUpdateCircuit) Define(api frontend.API) error {
	// 1. Enforce that the names are the same (only balance changes)
	for i := 0; i < NameLength; i++ {
		api.AssertIsEqual(c.OldUserState.NameBytes[i], c.NewUserState.NameBytes[i])
	}

    // 2. Compute old leaf hash from old user state
    oldLeafHash := hashUserStateCircuit(api, c.OldUserState)

    // 3. Compute new leaf hash from new user state
    newLeafHash := hashUserStateCircuit(api, c.NewUserState)

    // 4. Verify old root: recompute root from oldLeafHash, PathBits, and Siblings
    computedOldRoot := computeMerkleRoot(api, oldLeafHash, c.PathBits[:], c.Siblings[:])
    api.AssertIsEqual(computedOldRoot, c.OldRoot)

    // 5. Verify new root: recompute root from newLeafHash, same PathBits and Siblings
    computedNewRoot := computeMerkleRoot(api, newLeafHash, c.PathBits[:], c.Siblings[:])
    api.AssertIsEqual(computedNewRoot, c.NewRoot)

    // 6. Enforce state transition: newBalance = oldBalance + DepositAmount
    computedNewBalance := api.Add(c.OldUserState.Balance, c.DepositAmount)
    api.AssertIsEqual(computedNewBalance, c.NewUserState.Balance)

    return nil
}

// hashUserStateCircuit hashes name bytes and balance bytes using MiMC, matching off-chain hashing
func hashUserStateCircuit(api frontend.API, user UserStateCircuit) frontend.Variable {
    mimc, _ := mimc.NewMiMC(api)
    // Write name bytes
    for i := 0; i < NameLength; i++ {
        mimc.Write(user.NameBytes[i])
    }
    // Write balance bytes (32 bytes as in fr.Element.Bytes())
    for i := 0; i < 32; i++ {
        mimc.Write(user.BalanceBytes[i])
    }
    return mimc.Sum()
}

// computeMerkleRoot recomputes the Merkle root from a leaf and its proof
func computeMerkleRoot(api frontend.API, leaf frontend.Variable, pathBits []frontend.Variable, siblings []frontend.Variable) frontend.Variable {
    current := leaf
    for i := 0; i < len(siblings); i++ {
        isLeft := pathBits[i] // 1 if leaf is left child, 0 if right
        left := api.Select(isLeft, current, siblings[i])
        right := api.Select(isLeft, siblings[i], current)
        mimc, _ := mimc.NewMiMC(api)
        mimc.Write(left, right)
        current = mimc.Sum()
    }
    return current
}

// UserState holds a user's name and balance (same as wrappers)
type UserState struct {
	Name    string
	Balance *big.Int // TODO: Float
}

//--------------------------------------------------------------------------------
// Helper function: hashUserState
//
// Hashes a single user state (Name + Balance) into a field element using MiMC_BN254
// user can prove that the possess the same state by re-computing their claimed states
// and producing the same hash that in the state Merkle tree (Merkle proof).
//--------------------------------------------------------------------------------
func hashUserState(user UserState) *big.Int {
	// 1) Prepare a new MiMC hasher
	hasher := gcHash.MIMC_BN254.New()

	// 2) Convert user name (string) to bytes
	nameBytes := []byte(user.Name)
	// Write them into the hasher
	_, _ = hasher.Write(nameBytes)

	// 3) Convert user balance to fr.Element, then to bytes
	var balanceFr fr.Element
	balanceFr.SetBigInt(user.Balance)
	balanceBytes := balanceFr.Bytes()
	_, _ = hasher.Write(balanceBytes[:])

	// 4) Compute the digest
	digest := hasher.Sum(nil)

	// 5) Convert the resulting digest back into a big.Int
	var outFr fr.Element
	outFr.SetBytes(digest);
	res := new(big.Int)
	outFr.BigInt(res)
	return res
}
