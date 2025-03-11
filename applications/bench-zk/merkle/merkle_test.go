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
	root := buildMerkleStates(users)
	if root == nil {
		t.Fatal("Failed to build Merkle tree")
	}

	// Simulate an updated user state for Bob (after transaction)
	updatedUser := UserState{
		Name: "Bob",
		Ben:  big.NewInt(340),
	}

	updatedUserHash := hashUserState(updatedUser)
	if updatedUserHash == nil {
		t.Fatal("Failed to hash updated user state")
	}

	// Generate Merkle proof for Bob's updated state
	pr, err := generateMerkleProof(users, updatedUserHash)
	if err != nil {
		t.Fatalf("Error generating Merkle proof: %v", err)
	}

	// Verify the Merkle proof
	isValid := verifyMerkleProof(root, updatedUserHash, pr)
	if !isValid {
		t.Fatal("Merkle proof is invalid")
	}
}
