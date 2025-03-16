// circuit/circuit_test.go

package circuit

import (
	"math/big"
	"math/rand"
	"testing"
	"time"

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

	"bench-zk/merkle"
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
		BobOldBalance:  zero,   // Bob had 0
		BobNewBalance:  bobNew, // Bob now has 300
		SiblingBalance: zero,   // Alice's balance remains 0
	}

	// e) Full witness
	//----------------------------------------------------------------
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness: %v", err)
	}
	start := time.Now()
	//----------------------------------------------------------------

	//----------------------------------------------------------------
	// f) Generate the proof
	//----------------------------------------------------------------
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}
	proofTime := time.Since(start)

	start = time.Now()
	//----------------------------------------------------------------
	// g) Verify the proof with public inputs only
	//----------------------------------------------------------------
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness: %v", err)
	}
	verifyTime := time.Since(start)

	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("Failed to verify the proof: %v", err)
	}

	//----------------------------------------------------------------
	// If no error occurred, the test passed
	//----------------------------------------------------------------
	t.Log("Test passed successfully!")
	t.Logf("DepositCircuit: Depth=1, Leaves=2, Proof Generation Time=%v, Verification Time=%v", proofTime, verifyTime)
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
	start := time.Now()
	proofAzk, err := groth16.Prove(ccs, pk, fullWitnessA)
	if err != nil {
		t.Fatalf("Failed to generate proof for user A: %v", err)
	}
	proofTime := time.Since(start)

	// Step 10: Create public witness and verify proof for user A
	publicWitnessA, err := frontend.NewWitness(&assignmentA, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness for user A: %v", err)
	}
	start = time.Now()
	err = groth16.Verify(proofAzk, vk, publicWitnessA)
	if err != nil {
		t.Fatalf("Failed to verify proof for user A: %v", err)
	}
	verifyTime := time.Since(start)

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
	t.Logf("UserStateCircuit: Depth=1, Leaves=2, Proof Generation Time=%v, Verification Time=%v", proofTime, verifyTime)
}

// TestMerkleCircuit tests the MerkleCircuit with a 16-user, 4-level Merkle tree.
func TestMerkleCircuit(t *testing.T) {
	// Step 1: Initialize 16 users
	users := make([]merkle.UserState, 16)
	for i := 0; i < 16; i++ {
		users[i] = merkle.UserState{
			Name: big.NewInt(int64(i + 1)), // Names: 1 to 16
			Ben:  big.NewInt(100),          // Initial balance: 100 BEN each
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
	var pathBits [MD]frontend.Variable
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
		Siblings: [MD]frontend.Variable{
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
	start := time.Now()
	proofZk, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}
	proofTime := time.Since(start)

	// Step 11: Create public witness and verify proof
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness: %v", err)
	}
	start = time.Now()
	err = groth16.Verify(proofZk, vk, publicWitness)
	if err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
	verifyTime := time.Since(start)

	t.Log("Successfully verified MerkleCircuit proof for updating user 2's balance!")
	t.Logf("MerkleCircuit: Depth=4, Leaves=16, Proof Generation Time=%v, Verification Time=%v", proofTime, verifyTime)
}

func TestBatchMerkleCircuit(t *testing.T) {
	// Step 1: Initialize 16 users
	var initialLeaves [N]merkle.UserState
	for i := 0; i < N; i++ {
		initialLeaves[i] = merkle.UserState{
			Name: big.NewInt(int64(i + 1)), // Names: 1 to 16
			Ben:  big.NewInt(100),          // Initial balance: 100
		}
	}

	// Step 2: Compute the initial Merkle root
	start := time.Now()
	oldRoot := merkle.BuildMerkleStates(initialLeaves[:])

	// Step 3: Generate 32 random transactions
	var transactions [B]struct {
		LeafIndex     int
		DepositAmount *big.Int
	}
	currentLeaves := make([]merkle.UserState, N)
	copy(currentLeaves, initialLeaves[:])
	for k := 0; k < B; k++ {
		leafIndex := rand.Intn(N)                         // Random leaf: 0 to 15
		depositAmount := big.NewInt(int64(rand.Intn(11))) // Random deposit: 0 to 10
		transactions[k] = struct {
			LeafIndex     int
			DepositAmount *big.Int
		}{leafIndex, depositAmount}
		// Apply update off-circuit
		currentLeaves[leafIndex].Ben = new(big.Int).Add(currentLeaves[leafIndex].Ben, depositAmount)
	}

	// Step 4: Compute the final Merkle root
	newRoot := merkle.BuildMerkleStates(currentLeaves)

	// Step 5: Set up the circuit assignment
	var assignment BatchMerkleCircuit
	assignment.OldRoot = oldRoot
	assignment.NewRoot = newRoot
	for k := 0; k < B; k++ {
		assignment.Transactions[k].LeafIndex = big.NewInt(int64(transactions[k].LeafIndex))
		assignment.Transactions[k].DepositAmount = transactions[k].DepositAmount
	}
	for i := 0; i < N; i++ {
		assignment.InitialLeaves[i].Name = initialLeaves[i].Name
		assignment.InitialLeaves[i].Ben = initialLeaves[i].Ben
	}
	prepareTime := time.Since(start)

	// Step 6: Compile the circuit
	start = time.Now()
	var circuit BatchMerkleCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Failed to compile circuit: %v", err)
	}

	// Step 7: Setup proving and verifying keys
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("Failed to setup proving/verifying keys: %v", err)
	}

	// Step 8: Generate proof
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness: %v", err)
	}
	compileTime := time.Since(start)

	start = time.Now()
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}
	proofTime := time.Since(start)

	// Step 9: Verify proof
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness: %v", err)
	}
	start = time.Now()
	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
	verifyTime := time.Since(start)

	t.Logf("Successfully verified BatchMerkleCircuit proof for %v transactions!", B)
	t.Logf("BatchMerkleCircuit: Leaves=%d, Batch Size=%d, Proof Generation Time=%v, Verification Time=%v, Preparation Time=%v, Compile Time=%v", N, B, proofTime, verifyTime, prepareTime, compileTime)
}

// TestProofMerkleCircuit tests the ProofMerkleCircuit with adjustable D2 and B2.
func TestProofMerkleCircuit(t *testing.T) {
	const N2 = 1 << D2 // Number of leaves, e.g., 1024 for D2=10

	// Step 1: Initialize users
	var users [N2]merkle.UserState
	for i := 0; i < N2; i++ {
		users[i] = merkle.UserState{
			Name: big.NewInt(int64(i + 1)), // Names: 1 to N2
			Ben:  big.NewInt(100),          // Initial balance: 100
		}
	}

	// Step 2: Compute initial Merkle root
	oldRoot := merkle.BuildMerkleStates(users[:])

	// Step 3: Generate B2 transactions with proofs
	start := time.Now()
	type Transaction struct {
		LeafIndex     int
		DepositAmount *big.Int
		OldState      merkle.UserState
		NewState      merkle.UserState
		Proof         *merkle.MProof
	}
	var transactions [B2]Transaction
	currentUsers := make([]merkle.UserState, N2)
	copy(currentUsers, users[:])

	for k := 0; k < B2; k++ {
		leafIndex := rand.Intn(N2)
		oldState := currentUsers[leafIndex]
		H_old := merkle.HashUserState(oldState)
		proof, err := merkle.GenerateMerkleProof(currentUsers, H_old)
		if err != nil {
			t.Fatalf("Failed to generate Merkle proof for transaction %d: %v", k, err)
		}
		depositAmount := big.NewInt(int64(rand.Intn(11))) // Random deposit: 0-10
		newState := merkle.UserState{
			Name: oldState.Name,
			Ben:  new(big.Int).Add(oldState.Ben, depositAmount),
		}
		transactions[k] = Transaction{
			LeafIndex:     leafIndex,
			DepositAmount: depositAmount,
			OldState:      oldState,
			NewState:      newState,
			Proof:         proof,
		}
		currentUsers[leafIndex] = newState // Update state for next iteration
	}

	// Step 4: Compute final Merkle root
	newRoot := merkle.BuildMerkleStates(currentUsers)

	// Step 5: Assign circuit values
	var assignment ProofMerkleCircuit
	assignment.OldRoot = oldRoot
	assignment.NewRoot = newRoot
	for k := 0; k < B2; k++ {
		tx := transactions[k]
		assignment.Transactions[k].OldName = tx.OldState.Name
		assignment.Transactions[k].OldBalance = tx.OldState.Ben
		assignment.Transactions[k].NewName = tx.NewState.Name
		assignment.Transactions[k].NewBalance = tx.NewState.Ben
		for i := 0; i < D2; i++ {
			assignment.Transactions[k].Siblings[i] = tx.Proof.Siblings[i]
			if tx.Proof.PathBits[i] {
				assignment.Transactions[k].PathBits[i] = big.NewInt(1)
			} else {
				assignment.Transactions[k].PathBits[i] = big.NewInt(0)
			}
		}
	}
	prepareTime := time.Since(start)

	start = time.Now()
	// Step 6: Compile the circuit
	var circuit ProofMerkleCircuit
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatalf("Failed to compile circuit: %v", err)
	}

	// Step 7: Setup proving/verifying keys
	pk, vk, err := groth16.Setup(ccs)
	if err != nil {
		t.Fatalf("Failed to setup keys: %v", err)
	}

	// Step 8: Generate proof and measure time
	fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
	if err != nil {
		t.Fatalf("Failed to create full witness: %v", err)
	}
	compileTime := time.Since(start)

	start = time.Now()
	proof, err := groth16.Prove(ccs, pk, fullWitness)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}
	proofTime := time.Since(start)

	// Step 9: Verify proof
	publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
	if err != nil {
		t.Fatalf("Failed to create public witness: %v", err)
	}
	start = time.Now()
	err = groth16.Verify(proof, vk, publicWitness)
	if err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
	verifyTime := time.Since(start)

	t.Logf("ProofMerkleCircuit: Depth=%d, Leaves=%d, Batch Size=%d, Proof Generation Time=%v, Verification Time=%v, Preparation Time=%v, Compile Time=%v", D2, N2, B2, proofTime, verifyTime, prepareTime, compileTime)
}
