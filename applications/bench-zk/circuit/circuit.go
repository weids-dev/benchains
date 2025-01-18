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
