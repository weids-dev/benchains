// circuit/circuit_test.go

package circuit

import (
	"testing"
	"math/big"

	// ---------------------------
	//  GNARK libraries
	// ---------------------------
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"

	// ---------------------------
	//  GNARK-CRYPTO libraries
	// ---------------------------
	"github.com/consensys/gnark-crypto/ecc"
	"bench-zk/utils"
)

// TestDepositCircuit tests the entire flow of circuit compilation, proving, and verification
func TestDepositCircuit(t *testing.T) {
	//----------------------------------------------------------------
	// a) Construct the circuit constraints shape
	//----------------------------------------------------------------
	var circuit DepositCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Failed to compile circuit: %v", err)
	}

	//----------------------------------------------------------------
	// b) Setup: proving key, verifying key
	//----------------------------------------------------------------
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("Failed to setup proving/verifying keys: %v", err)
	}

	//----------------------------------------------------------------
	// c) Off-circuit: we have
	//    oldRoot = MiMC(0, 0)
	//    newRoot = MiMC(300, 0)
	//    deposit = 300
	//----------------------------------------------------------------
	zero := big.NewInt(0)
	oldRootInt := utils.ComputeMiMC(zero, zero)

	deposit := big.NewInt(300)
	bobNew := big.NewInt(300)
	newRootInt := utils.ComputeMiMC(bobNew, zero)

	//----------------------------------------------------------------
	// d) Build the assignment that satisfies the circuit
	//----------------------------------------------------------------
	assignment := DepositCircuit{
		// public
		OldRoot:       oldRootInt,
		NewRoot:       newRootInt,
		DepositAmount: deposit,

		// private
		BobOldBalance: zero,   // Bob had 0
		BobNewBalance: bobNew, // Bob now has 300
		SiblingBalance: zero,  // Alice's balance remains 0
	}

	//----------------------------------------------------------------
	// e) Full witness
	//----------------------------------------------------------------
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness: %v", err)
	}

	//----------------------------------------------------------------
	// f) Generate the proof
	//----------------------------------------------------------------
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	//----------------------------------------------------------------
	// g) Verify the proof with public inputs only
	//----------------------------------------------------------------
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness: %v", err)
	}

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("Failed to verify the proof: %v", err)
	}

	//----------------------------------------------------------------
	// If no error occurred, the test passed
	//----------------------------------------------------------------
	t.Log("Test passed successfully!")
}
