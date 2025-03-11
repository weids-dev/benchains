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
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
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

func TestMerkleUpdateCircuit(t *testing.T) {
    // Define users (same as in merkle_test.go)
    users := []merkle.UserState{
        {"Alice", big.NewInt(100)},
        {"Bob", big.NewInt(340)},
        {"Charlie", big.NewInt(500)},
        {"David", big.NewInt(750)},
        {"Eva", big.NewInt(200)},
        {"Frank", big.NewInt(900)},
        {"Grace", big.NewInt(50)},
        {"Hannah", big.NewInt(1200)},
        {"Isaac", big.NewInt(180)},
        {"Jack", big.NewInt(350)},
        {"Kathy", big.NewInt(450)},
        {"Leo", big.NewInt(600)},
        {"Mona", big.NewInt(800)},
        {"Nina", big.NewInt(150)},
        {"Oscar", big.NewInt(1100)},
        {"Paul", big.NewInt(950)},
        {"Quinn", big.NewInt(300)},
        {"Rita", big.NewInt(400)},
        {"Steve", big.NewInt(550)},
        {"Tina", big.NewInt(50)},
        {"Victor", big.NewInt(720)},
        {"Wendy", big.NewInt(670)},
        {"Xander", big.NewInt(90)},
        {"Yara", big.NewInt(1000)},
    }

    // Build initial Merkle tree
    initialRoot := merkle.BuildMerkleStates(users)
    if initialRoot == nil {
        t.Fatal("Failed to build initial Merkle tree")
    }

    // Select Bob (index 1)
    bobIndex := 1
    bobOldState := users[bobIndex]
    bobOldLeafHash := merkle.HashUserState(bobOldState)

    // Generate Merkle proof for Bob's old state
    proof, err := merkle.GenerateMerkleProof(users, bobOldLeafHash)
    if err != nil {
        t.Fatalf("Error generating Merkle proof: %v", err)
    }

    // Define deposit amount
    depositAmount := big.NewInt(20)

    // Compute new balance and state
    bobNewBalance := new(big.Int).Add(bobOldState.Ben, depositAmount)
    bobNewState := merkle.UserState{Name: bobOldState.Name, Ben: bobNewBalance}

    // Compute new root off-chain
    newRoot := merkle.UpdateMerkleRoot(proof, bobNewState)

    // Prepare circuit inputs
    nameBytes := padNameToBytes(bobOldState.Name)

    var oldBalanceFr fr.Element
    oldBalanceFr.SetBigInt(bobOldState.Ben)
    oldBalanceBytes := oldBalanceFr.Bytes()

    var newBalanceFr fr.Element
    newBalanceFr.SetBigInt(bobNewBalance)
    newBalanceBytes := newBalanceFr.Bytes()

    var pathBits [TreeDepth]frontend.Variable
    var siblings [TreeDepth]frontend.Variable
    if len(proof.PathBits) != TreeDepth || len(proof.Siblings) != TreeDepth {
        t.Fatalf("Proof length %d does not match TreeDepth %d", len(proof.PathBits), TreeDepth)
    }
    for i := 0; i < TreeDepth; i++ {
        pathBits[i] = 0
        if proof.PathBits[i] {
            pathBits[i] = 1
        }
        siblings[i] = proof.Siblings[i]
    }

    // Create assignment
    var oldBalanceBytesArray [32]frontend.Variable
    var newBalanceBytesArray [32]frontend.Variable
    for i := 0; i < 32; i++ {
        oldBalanceBytesArray[i] = big.NewInt(int64(oldBalanceBytes[i]))
        newBalanceBytesArray[i] = big.NewInt(int64(newBalanceBytes[i]))
    }

    assignment := MerkleUpdateCircuit{
        OldRoot:       initialRoot,
        NewRoot:       newRoot,
        DepositAmount: depositAmount,
        OldUserState: UserStateCircuit{
            NameBytes:    nameBytes,
            Balance:      bobOldState.Ben,
            BalanceBytes: oldBalanceBytesArray,
        },
        NewUserState: UserStateCircuit{
            NameBytes:    nameBytes,
            Balance:      bobNewBalance,
            BalanceBytes: newBalanceBytesArray,
        },
        PathBits: pathBits,
        Siblings: siblings,
    }

    // Compile circuit
    var circuit MerkleUpdateCircuit
    ccs, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, &circuit)
    if err != nil {
        t.Fatalf("Failed to compile circuit: %v", err)
    }

    // Setup proving and verifying keys
    pk, vk, err := groth16.Setup(ccs)
    if err != nil {
        t.Fatalf("Failed to setup keys: %v", err)
    }

    // Generate full witness
    fullWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField())
    if err != nil {
        t.Fatalf("Failed to create full witness: %v", err)
    }

    // Generate proof
    proofGroth16, err := groth16.Prove(ccs, pk, fullWitness)
    if err != nil {
        t.Fatalf("Failed to generate proof: %v", err)
    }

    // Generate public witness
    publicWitness, err := frontend.NewWitness(&assignment, ecc.BN254.ScalarField(), frontend.PublicOnly())
    if err != nil {
        t.Fatalf("Failed to create public witness: %v", err)
    }

    // Verify proof
    err = groth16.Verify(proofGroth16, vk, publicWitness)
    if err != nil {
        t.Fatalf("Failed to verify proof: %v", err)
    }

    t.Log("MerkleUpdateCircuit test passed successfully!")
}

// padNameToBytes converts a name to a fixed-length byte array
func padNameToBytes(name string) [NameLength]frontend.Variable {
    bytes := []byte(name)
    var padded [NameLength]frontend.Variable
    for i := 0; i < NameLength; i++ {
        if i < len(bytes) {
            padded[i] = big.NewInt(int64(bytes[i]))
        } else {
            padded[i] = big.NewInt(0)
        }
    }
    return padded
}
