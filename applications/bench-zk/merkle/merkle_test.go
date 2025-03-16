// merkle/merkle_test.go

package merkle

import (
	"math/big"
	"testing"
)

func TestMerkleProof(t *testing.T) {
	// Sample user states for testing
	users := []UserState{
		{new(big.Int).SetBytes([]byte("Alice")), big.NewInt(100)},
		{new(big.Int).SetBytes([]byte("Bob")), big.NewInt(340)},
		{new(big.Int).SetBytes([]byte("Charlie")), big.NewInt(500)},
		{new(big.Int).SetBytes([]byte("David")), big.NewInt(750)},
		{new(big.Int).SetBytes([]byte("Eva")), big.NewInt(200)},
		{new(big.Int).SetBytes([]byte("Frank")), big.NewInt(900)},
		{new(big.Int).SetBytes([]byte("Grace")), big.NewInt(50)},
		{new(big.Int).SetBytes([]byte("Hannah")), big.NewInt(1200)},
		{new(big.Int).SetBytes([]byte("Isaac")), big.NewInt(180)},
		{new(big.Int).SetBytes([]byte("Jack")), big.NewInt(350)},
		{new(big.Int).SetBytes([]byte("Kathy")), big.NewInt(450)},
		{new(big.Int).SetBytes([]byte("Leo")), big.NewInt(600)},
		{new(big.Int).SetBytes([]byte("Mona")), big.NewInt(800)},
		{new(big.Int).SetBytes([]byte("Nina")), big.NewInt(150)},
		{new(big.Int).SetBytes([]byte("Oscar")), big.NewInt(1100)},
		{new(big.Int).SetBytes([]byte("Paul")), big.NewInt(950)},
		{new(big.Int).SetBytes([]byte("Quinn")), big.NewInt(300)},
		{new(big.Int).SetBytes([]byte("Rita")), big.NewInt(400)},
		{new(big.Int).SetBytes([]byte("Steve")), big.NewInt(550)},
		{new(big.Int).SetBytes([]byte("Tina")), big.NewInt(50)},
		{new(big.Int).SetBytes([]byte("Victor")), big.NewInt(720)},
		{new(big.Int).SetBytes([]byte("Wendy")), big.NewInt(670)},
		{new(big.Int).SetBytes([]byte("Xander")), big.NewInt(90)},
		{new(big.Int).SetBytes([]byte("Yara")), big.NewInt(1000)},
	}

	// Build the Merkle tree and retrieve the root
	root := BuildMerkleStates(users)
	if root == nil {
		t.Fatal("Failed to build Merkle tree")
	}

	// Simulate an updated user state for Bob (after transaction)
	updatedUser := UserState{
		Name: new(big.Int).SetBytes([]byte("Bob")),
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
		{new(big.Int).SetBytes([]byte("Alice")), big.NewInt(100)},
		{new(big.Int).SetBytes([]byte("Bob")), big.NewInt(340)},
		{new(big.Int).SetBytes([]byte("Charlie")), big.NewInt(500)},
		{new(big.Int).SetBytes([]byte("David")), big.NewInt(750)},
		{new(big.Int).SetBytes([]byte("Eva")), big.NewInt(200)},
		{new(big.Int).SetBytes([]byte("Frank")), big.NewInt(900)},
		{new(big.Int).SetBytes([]byte("Grace")), big.NewInt(50)},
		{new(big.Int).SetBytes([]byte("Hannah")), big.NewInt(1200)},
		{new(big.Int).SetBytes([]byte("Isaac")), big.NewInt(180)},
		{new(big.Int).SetBytes([]byte("Jack")), big.NewInt(350)},
		{new(big.Int).SetBytes([]byte("Kathy")), big.NewInt(450)},
		{new(big.Int).SetBytes([]byte("Leo")), big.NewInt(600)},
		{new(big.Int).SetBytes([]byte("Mona")), big.NewInt(800)},
		{new(big.Int).SetBytes([]byte("Nina")), big.NewInt(150)},
		{new(big.Int).SetBytes([]byte("Oscar")), big.NewInt(1100)},
		{new(big.Int).SetBytes([]byte("Paul")), big.NewInt(950)},
		{new(big.Int).SetBytes([]byte("Quinn")), big.NewInt(300)},
		{new(big.Int).SetBytes([]byte("Rita")), big.NewInt(400)},
		{new(big.Int).SetBytes([]byte("Steve")), big.NewInt(550)},
		{new(big.Int).SetBytes([]byte("Tina")), big.NewInt(50)},
		{new(big.Int).SetBytes([]byte("Victor")), big.NewInt(720)},
		{new(big.Int).SetBytes([]byte("Wendy")), big.NewInt(670)},
		{new(big.Int).SetBytes([]byte("Xander")), big.NewInt(90)},
		{new(big.Int).SetBytes([]byte("Yara")), big.NewInt(1000)},
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
