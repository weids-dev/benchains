package heain

import (
	"fmt"
	"math/big"

	// ---------------------------
	//  GNARK libraries
	// ---------------------------
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/hash/mimc"

	// ---------------------------
	//  GNARK-CRYPTO libraries
	// ---------------------------
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	gcHash "github.com/consensys/gnark-crypto/hash"
)

//--------------------------------------------------------------------------------
// 1) Define the circuit
//--------------------------------------------------------------------------------

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
	fmt.Print(c.NewRoot)
	fmt.Println()
	fmt.Print(computedNewRoot)

	return nil
}

//--------------------------------------------------------------------------------
// 2) Helper function to do off-circuit MiMC(BN254) using gnark-crypto
//--------------------------------------------------------------------------------

// computeMiMC takes two big.Ints (representing BobBalance, SiblingBalance),
// writes them into a gnark-crypto MiMC_BN254 hasher, and returns the resulting
// big.Int (field element) that matches exactly what in-circuit MiMC(BN254) computes.

func computeMiMC(b1, b2 *big.Int) *big.Int {
	// 1) Convert big.Int → fr.Element
	var e1, e2 fr.Element
	e1.SetBigInt(b1)
	e2.SetBigInt(b2)

	// 2) Extract their bytes ([32]byte) then slice them
	e1Bytes := e1.Bytes()
	e2Bytes := e2.Bytes()

	// 3) Create a new MiMC_BN254 hasher
	hasher := gcHash.MIMC_BN254.New()

	// 4) Write both field-element slices into the hasher
	_, _ = hasher.Write(e1Bytes[:])
	_, _ = hasher.Write(e2Bytes[:])

	// 5) Sum → digest ([]byte)
	digest := hasher.Sum(nil)

	// 6) Convert digest back to an fr.Element
	var outFr fr.Element
	outFr.SetBytes(digest);

	// 7) Convert fr.Element → *big.Int
	res := new(big.Int)
	outFr.BigInt(res)
	return res
}



//--------------------------------------------------------------------------------
// 3) main: assemble the example
//--------------------------------------------------------------------------------
func main() {

	//----------------------------------------------------------------
	// a) Construct the circuit constraints shape
	//----------------------------------------------------------------
	var circuit DepositCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		panic(err)
	}

	//----------------------------------------------------------------
	// b) Setup: proving key, verifying key
	//----------------------------------------------------------------
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		panic(err)
	}

	//----------------------------------------------------------------
	// c) Off-circuit: we have
	//    oldRoot = MiMC(0, 0)
	//    newRoot = MiMC(300, 0)
	//    deposit = 300
	//----------------------------------------------------------------
	zero := big.NewInt(0)
	oldRootInt := computeMiMC(zero, zero)

	deposit := big.NewInt(300)
	bobNew := big.NewInt(300)
	newRootInt := computeMiMC(bobNew, zero)

	fmt.Printf("oldRoot = %s\n", oldRootInt.String())
	fmt.Printf("newRoot = %s\n", newRootInt.String())

	//----------------------------------------------------------------
	// d) Build the assignment that satisfies the circuit
	//----------------------------------------------------------------
	assignment := DepositCircuit{
		// public
		OldRoot:       oldRootInt,
		NewRoot:       newRootInt,
		DepositAmount: deposit,

		// private
		BobOldBalance:  zero,   // Bob had 0
		BobNewBalance:  bobNew, // Bob now has 300
		SiblingBalance: zero,   // Alice's balance remains 0
	}

	//----------------------------------------------------------------
	// e) Full witness
	//----------------------------------------------------------------
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		panic(err)
	}

	//----------------------------------------------------------------
	// f) Generate the proof
	//----------------------------------------------------------------
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		panic(err)
	}

	//----------------------------------------------------------------
	// g) Verify the proof with public inputs only
	//----------------------------------------------------------------
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		panic(err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		fmt.Println("INVALID proof:", err)
	} else {
		fmt.Println("Proof is CORRECT!")
	}
}
