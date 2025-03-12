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
    // "github.com/consensys/gnark-crypto/ecc/bn254/fr"

	"bench-zk/utils"
	"bench-zk/merkle"
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


// TestUserStateCircuit tests the UserStateCircuit with a two-user, one-level Merkle tree.
func TestUserStateCircuit(t *testing.T) {
	// Step 1: Define two initial users
	userA := merkle.UserState{Name: big.NewInt(1), Ben: big.NewInt(100)} // User A: Name=1, Balance=100
	userB := merkle.UserState{Name: big.NewInt(2), Ben: big.NewInt(200)} // User B: Name=2, Balance=200
	users := []merkle.UserState{userA, userB}

	// Step 2: Compute their state hashes
	H_A := merkle.HashUserState(userA)
	H_B := merkle.HashUserState(userB)

	// Step 3: Compute the old Merkle root
	oldRoot := utils.ComputeMiMC(H_A, H_B)

	// Step 4: Update user A’s state (e.g., increase balance by 50)
	userAUpdated := merkle.UserState{Name: big.NewInt(1), Ben: big.NewInt(150)}

	// Step 5: Generate Merkle proof for user A and compute new root
	proofA, err := merkle.GenerateMerkleProof(users, H_A)
	if err != nil {
		t.Fatalf("Failed to generate Merkle proof for user A: %v", err)
	}
	newRootA := merkle.UpdateMerkleRoot(proofA, userAUpdated)

	// Step 6: Set up the circuit assignment (updating left leaf, user A)
	assignmentA := UserStateCircuit{
		OldRoot:     oldRoot,
		NewRoot:     newRootA,
		OldName:     userA.Name,
		OldBalance:  userA.Ben,
		NewName:     userAUpdated.Name,
		NewBalance:  userAUpdated.Ben,
		SiblingHash: H_B,
		PathBit:     1, // Left leaf
	}

	// Step 7: Compile the circuit
	var circuit UserStateCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Failed to compile circuit: %v", err)
	}

	// Step 8: Setup proving and verifying keys
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("Failed to setup proving/verifying keys: %v", err)
	}

	// Step 9: Create full witness and generate proof for user A
	fullWitnessA, err := frontend.NewWitness(&assignmentA, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness for user A: %v", err)
	}
	proofAzk, err := groth16.Prove(ccs, pk, fullWitnessA)
	if err != nil {
		t.Fatalf("Failed to generate proof for user A: %v", err)
	}

	// Step 10: Create public witness and verify proof for user A
	publicWitnessA, err := frontend.NewWitness(&assignmentA, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness for user A: %v", err)
	}
	err = groth16.Verify(proofAzk, vk, publicWitnessA)
	if err != nil {
		t.Fatalf("Failed to verify proof for user A: %v", err)
	}

	// Step 11: Test updating user B (right leaf)
	userBUpdated := merkle.UserState{Name: big.NewInt(2), Ben: big.NewInt(250)}
	proofB, err := merkle.GenerateMerkleProof(users, H_B)
	if err != nil {
		t.Fatalf("Failed to generate Merkle proof for user B: %v", err)
	}
	newRootB := merkle.UpdateMerkleRoot(proofB, userBUpdated)

	assignmentB := UserStateCircuit{
		OldRoot:     oldRoot,
		NewRoot:     newRootB,
		OldName:     userB.Name,
		OldBalance:  userB.Ben,
		NewName:     userBUpdated.Name,
		NewBalance:  userBUpdated.Ben,
		SiblingHash: H_A,
		PathBit:     0, // Right leaf
	}

	// Step 12: Create full witness and generate proof for user B
	fullWitnessB, err := frontend.NewWitness(&assignmentB, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness for user B: %v", err)
	}
	proofBzk, err := groth16.Prove(ccs, pk, fullWitnessB)
	if err != nil {
		t.Fatalf("Failed to generate proof for user B: %v", err)
	}

	// Step 13: Create public witness and verify proof for user B
	publicWitnessB, err := frontend.NewWitness(&assignmentB, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness for user B: %v", err)
	}
	err = groth16.Verify(proofBzk, vk, publicWitnessB)
	if err != nil {
		t.Fatalf("Failed to verify proof for user B: %v", err)
	}

	t.Log("Test passed successfully for both user A and user B updates!")
}

// TestMerkleCircuit tests the MerkleCircuit with a 16-user, 4-level Merkle tree.
func TestMerkleCircuit(t *testing.T) {
	// Step 1: Initialize 16 users
	users := make([]merkle.UserState, 16)
	for i := 0; i < 16; i++ {
		users[i] = merkle.UserState{
			Name: big.NewInt(int64(i + 1)),     // Names: 1 to 16
			Ben:  big.NewInt(100),              // Initial balance: 100 BEN each
		}
	}

	// Step 2: Compute the old Merkle root
	oldRoot := merkle.BuildMerkleStates(users)

	// Step 3: Choose user 2 (index 1) and generate Merkle proof for their old state
	userIndex := 1
	oldUser := users[userIndex]
	H_old := merkle.HashUserState(oldUser)
	proof, err := merkle.GenerateMerkleProof(users, H_old)
	if err != nil {
		t.Fatalf("Failed to generate Merkle proof for user %d: %v", userIndex+1, err)
	}

	// Step 4: Update user 2’s state (deposit 20 BEN)
	newUser := merkle.UserState{
		Name: oldUser.Name,
		Ben:  new(big.Int).Add(oldUser.Ben, big.NewInt(20)), // 100 + 20 = 120 BEN
	}

	// Step 5: Compute the new Merkle root
	newRoot := merkle.UpdateMerkleRoot(proof, newUser)

	// Step 6: Prepare PathBits as *big.Int for the circuit
	var pathBits [D]frontend.Variable
	for i, b := range proof.PathBits {
		if b {
			pathBits[i] = big.NewInt(1)
		} else {
			pathBits[i] = big.NewInt(0)
		}
	}

	// Step 7: Set up the circuit assignment
	assignment := MerkleCircuit{
		OldRoot:    oldRoot,
		NewRoot:    newRoot,
		OldName:    oldUser.Name,
		OldBalance: oldUser.Ben,
		NewName:    newUser.Name,
		NewBalance: newUser.Ben,
		Siblings:   [D]frontend.Variable{
			proof.Siblings[0],
			proof.Siblings[1],
			proof.Siblings[2],
			proof.Siblings[3],
		},
		PathBits: pathBits,
	}

	// Step 8: Compile the circuit
	var circuit MerkleCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Failed to compile circuit: %v", err)
	}

	// Step 9: Setup proving and verifying keys
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("Failed to setup proving/verifying keys: %v", err)
	}

	// Step 10: Create full witness and generate proof
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness: %v", err)
	}
	proofZk, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	// Step 11: Create public witness and verify proof
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness: %v", err)
	}
	err = groth16.Verify(proofZk, vk, publicWitness)
	if err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}

	t.Log("Successfully verified MerkleCircuit proof for updating user 2's balance!")
}
