// merkle/merkle_test.go

package merkle

import (
	"math/big"
	"testing"
)

func TestMerkleProof(t *testing.T) {
	// Sample user states for testing
	users := []UserState{
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

	// Build the Merkle tree and retrieve the root
	root := BuildMerkleStates(users)
	if root == nil {
		t.Fatal("Failed to build Merkle tree")
	}

	// Simulate an updated user state for Bob (after transaction)
	updatedUser := UserState{
		Name: "Bob",
		Ben:  big.NewInt(340),
	}

	updatedUserHash := HashUserState(updatedUser)
	if updatedUserHash == nil {
		t.Fatal("Failed to hash updated user state")
	}

	// Generate Merkle proof for Bob's updated state
	pr, err := GenerateMerkleProof(users, updatedUserHash)
	if err != nil {
		t.Fatalf("Error generating Merkle proof: %v", err)
	}

	// Verify the Merkle proof
	isValid := VerifyMerkleProof(root, updatedUserHash, pr)
	if !isValid {
		t.Fatal("Merkle proof is invalid")
	}
}

func TestMerkleUpdate(t *testing.T) {
	// Initialize a list of users (same as TestMerkleProof for consistency)
	users := []UserState{
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

    // Build the initial Merkle tree
    initialRoot := BuildMerkleStates(users)
    if initialRoot == nil {
        t.Fatal("Failed to build initial Merkle tree")
    }

    // Select Bob (index 1) and get his previous state
    bobIndex := 1
    bobPrevState := users[bobIndex]
    bobPrevLeafHash := HashUserState(bobPrevState)

    // Generate Merkle proof for Bob's previous state
    prevProof, err := GenerateMerkleProof(users, bobPrevLeafHash)
    if err != nil {
        t.Fatalf("Error generating previous Merkle proof: %v", err)
    }

    // Verify the previous proof (optional sanity check)
    if !VerifyMerkleProof(initialRoot, bobPrevLeafHash, prevProof) {
        t.Fatal("Previous Merkle proof is invalid for initial root")
    }

    // Update Bob's balance (increase by 20, e.g., 340 to 360)
    newBalance := new(big.Int).Add(bobPrevState.Ben, big.NewInt(20))
    bobNewState := UserState{Name: bobPrevState.Name, Ben: newBalance}

    // Compute the new root and proof
    newRoot := UpdateMerkleRoot(prevProof, bobNewState)

    // Compute the new leaf hash for verification
    newLeafHash := HashUserState(bobNewState)

    // Simulate circuit verification: check if the new proof is valid for the new root
	if !VerifyMerkleProof(newRoot, newLeafHash, prevProof) {
        t.Fatal("New Merkle proof is invalid for the updated state")
    }

    // Additional validation: rebuild the entire tree with the updated state
    updatedUsers := make([]UserState, len(users))
    copy(updatedUsers, users)
    updatedUsers[bobIndex] = bobNewState
    actualNewRoot := BuildMerkleStates(updatedUsers)
    if actualNewRoot == nil {
        t.Fatal("Failed to build updated Merkle tree")
    }

    // Check if the computed new root matches the actual new root
    if newRoot.Cmp(actualNewRoot) != 0 {
        t.Fatalf("Computed new root %v does not match actual new root %v", newRoot, actualNewRoot)
    }
}
