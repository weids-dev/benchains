package merkle

import (
	"fmt"
	"math/big"

	"bench-zk/utils"
	gcHash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// UserState holds a user's name and balance
type UserState struct {
	Name    string
	Balance *big.Int // TODO: Float
}

// MProof represents the Merkle proof for a specific leaf in the Merkle tree.
type MProof struct {
	PathBits []bool      // Indicates the direction (left or right) at each level of the tree
	Siblings []*big.Int  // Contains the sibling hashes at each level of the tree
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

//--------------------------------------------------------------------------------
// Helper function: buildMerkleStates
//
// Builds a (very naive) Merkle tree from a slice of UserState.
// Each leaf = hashUserState(user.Name, user.Balance).
// Then pairwise hash to get parent, etc. Returns the Merkle root as *big.Int.
//--------------------------------------------------------------------------------
func buildMerkleStates(users []UserState) *big.Int {
	// 1) Hash each user into a leaf
	var leaves []*big.Int
	for _, u := range users {
		leaf := hashUserState(u)
		leaves = append(leaves, leaf)
	}

	// Edge case: if no users, return 0
	if len(leaves) == 0 {
		return big.NewInt(0)
	}

	// 2) Build up the tree by pairwise hashing
	for len(leaves) > 1 {
		var nextLevel []*big.Int

		for i := 0; i < len(leaves); i += 2 {
			// if odd number of leaves, carry the last leaf over if it has no pair
			if i+1 == len(leaves) {
				nextLevel = append(nextLevel, leaves[i])
			} else {
				parent := utils.ComputeMiMC(leaves[i], leaves[i+1])
				nextLevel = append(nextLevel, parent)
			}
		}

		leaves = nextLevel
	}

	// 3) The single element left is the root
	return leaves[0]
}


// generateMerkleProof generates a Merkle proof for the given leaf in the tree.
func generateMerkleProof(users []UserState, leaf *big.Int) (*MProof, error) {
	// 1) Hash each user into a leaf
	var leaves []*big.Int
	for _, u := range users {
		leafHash := hashUserState(u)
		leaves = append(leaves, leafHash)
	}

	// Edge case: empty tree, no proof
	if len(leaves) == 0 {
		return nil, fmt.Errorf("empty tree, no proof available")
	}

	// Initialize proof structure
	proof := &MProof{
		PathBits: []bool{},
		Siblings: []*big.Int{},
	}

	// 2) Traverse the tree from leaf to root
	for len(leaves) > 1 {
		var nextLevel []*big.Int
		for i := 0; i < len(leaves); i += 2 {
			// if odd number of leaves, carry the last leaf over
			if i+1 == len(leaves) {
				nextLevel = append(nextLevel, leaves[i])
				continue
			}
			// Determine the direction and compute the parent node
			if leaves[i].String() == leaf.String() {
				proof.PathBits = append(proof.PathBits, true) // Left to right
				proof.Siblings = append(proof.Siblings, leaves[i+1])
				newleaf := utils.ComputeMiMC(leaves[i], leaves[i+1])
				nextLevel = append(nextLevel, newleaf)
				leaf = newleaf
			} else if leaves[i+1].String() == leaf.String() {
				proof.PathBits = append(proof.PathBits, false) // Right to left
				proof.Siblings = append(proof.Siblings, leaves[i])
				newleaf := utils.ComputeMiMC(leaves[i], leaves[i+1])
				nextLevel = append(nextLevel, newleaf)
				leaf = newleaf
			} else {
				nextLevel = append(nextLevel, utils.ComputeMiMC(leaves[i], leaves[i+1]))
			}
		}
		leaves = nextLevel
	}

	// Return the Merkle proof
	return proof, nil
}


// verifyMerkleProof verifies that the provided proof is valid for the given root and leaf.
func verifyMerkleProof(root *big.Int, leaf *big.Int, proof *MProof) bool {
	// Start with the leaf hash
	currentHash := leaf

	// Traverse the proof and rebuild the path to the root
	for i, sibling := range proof.Siblings {
		if proof.PathBits[i] {
			// Left to right: hash(currentHash || sibling)
			currentHash = utils.ComputeMiMC(currentHash, sibling)
		} else {
			// Right to left: hash(sibling || currentHash)
			currentHash = utils.ComputeMiMC(sibling, currentHash)
		}
	}

	// Check if the final hash matches the root
	return currentHash.Cmp(root) == 0
}
