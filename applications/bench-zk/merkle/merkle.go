// merkle/merkle.go

package merkle

import (
	"fmt"
	"math/big"
	"encoding/base64"

	"bench-zk/utils"

	gcHash "github.com/consensys/gnark-crypto/hash"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

// MProof represents the Merkle proof for a specific leaf in the Merkle tree.
type MProof struct {
	PathBits []bool      // Indicates the direction (left or right) at each level of the tree
	Siblings []*big.Int  // Contains the sibling hashes at each level of the tree
}

// UserState holds a user's name, balance ($BEN).
// Each UserState will be commit to the transaction root every period of time.
// When exiting, user can withdraw all BEN stored on zk-rollups contract chain,
// but he/she cannot move any unused deposits out.
type UserState struct {
	Name     string
	Ben      *big.Int
}


// TransactionData holds the transaction ID and its corresponding args
type TransactionData struct {
	TxID string
	Args []string
}


//--------------------------------------------------------------------------------
// Helper function: HashUserState
//
// Hashes a single user state (Name + Balance) into a field element using MiMC_BN254
// user can prove that the possess the same state by re-computing their claimed states
// and producing the same hash that in the state Merkle tree (Merkle proof).
//--------------------------------------------------------------------------------

func HashUserState(user UserState) *big.Int {
	hasher := gcHash.MIMC_BN254.New()

	NameLength := 10
    // Pad name to NameLength (10) bytes
    nameBytes := []byte(user.Name)
    if len(nameBytes) > NameLength {
        nameBytes = nameBytes[:NameLength]
    } else {
        for len(nameBytes) < NameLength {
            nameBytes = append(nameBytes, 0)
        }
    }
    _, _ = hasher.Write(nameBytes)

    // Convert balance to fr.Element bytes
    var balanceFr fr.Element
    balanceFr.SetBigInt(user.Ben)
    balanceBytes := balanceFr.Bytes()
    _, _ = hasher.Write(balanceBytes[:])

    digest := hasher.Sum(nil)
    var outFr fr.Element
    outFr.SetBytes(digest)
    res := new(big.Int)
    outFr.BigInt(res)
    return res
}

// hashTransactionData hashes a single transaction (TxID + all Args)
// into a field element using MiMC_BN254.
//
// The order is:
//   1. TxID (as bytes) first,
//   2. Then each argument (in sequence).
func HashTransactionData(tx TransactionData) *big.Int {
	hasher := gcHash.MIMC_BN254.New()

	// 1) Write TxID bytes
	txIDBytes := []byte(tx.TxID)
	_, _ = hasher.Write(txIDBytes)

	// 2) Write each argument’s bytes
	for _, arg := range tx.Args {
		argBytes := []byte(arg)
		_, _ = hasher.Write(argBytes)
	}

	// 3) Compute the hash
	digest := hasher.Sum(nil)

	// 4) Convert to fr.Element => big.Int
	var outFr fr.Element
	outFr.SetBytes(digest)

	res := new(big.Int)
	outFr.BigInt(res)
	return res
}

// MerkleRootToBase64 takes a big.Int (Merkle root) and encodes it in Base64.
// This string can then be committed to a blockchain or stored anywhere you need text representation.
func MerkleRootToBase64(root *big.Int) string {
	// 1) Convert big.Int to a big-endian byte slice
	rootBytes := root.Bytes()
	
	// 2) Encode to base64
	encoded := base64.StdEncoding.EncodeToString(rootBytes)
	
	return encoded
}

// BuildMerkleTransactions takes a list of transactions, 
// hashes each transaction to produce leaves, and then builds 
// a Merkle tree using pairwise MiMC hashing. It returns the Merkle root as *big.Int.
func BuildMerkleTransactions(txs []TransactionData) *big.Int {
	// 1) Create a leaf for each transaction by hashing it.
	var leaves []*big.Int
	for _, tx := range txs {
		leaf := HashTransactionData(tx)
		leaves = append(leaves, leaf)
	}

	// Edge case: if no transactions, return 0
	if len(leaves) == 0 {
		return big.NewInt(0)
	}

	// 2) Build up the tree by pairwise hashing
	for len(leaves) > 1 {
		var nextLevel []*big.Int

		for i := 0; i < len(leaves); i += 2 {
			// if odd number of leaves, carry over the last one if it has no pair
			if i+1 == len(leaves) {
				nextLevel = append(nextLevel, leaves[i])
			} else {
				// parent = MiMC(leaves[i], leaves[i+1])
				parent := utils.ComputeMiMC(leaves[i], leaves[i+1])
				nextLevel = append(nextLevel, parent)
			}
		}
		leaves = nextLevel
	}

	// 3) At the end, leaves[0] is the Merkle root
	return leaves[0]
}


//--------------------------------------------------------------------------------
// Helper function: BuildMerkleStates
//
// Builds a (very naive) Merkle tree from a slice of UserState.
// Each leaf = HashUserState(user.Name, user.Balance).
// Then pairwise hash to get parent, etc. Returns the Merkle root as *big.Int.
//--------------------------------------------------------------------------------
func BuildMerkleStates(users []UserState) *big.Int {
	// 1) Hash each user into a leaf
	var leaves []*big.Int
	for _, u := range users {
		leaf := HashUserState(u)
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

// GenerateMerkleProof generates a Merkle proof for the given leaf in the tree.
func GenerateMerkleProof(users []UserState, leaf *big.Int) (*MProof, error) {
	// 1) Hash each user into a leaf
	var leaves []*big.Int
	for _, u := range users {
		leafHash := HashUserState(u)
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

// VerifyMerkleProof verifies that the provided proof is valid for the given root and leaf.
func VerifyMerkleProof(root *big.Int, leaf *big.Int, proof *MProof) bool {
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


// UpdateMerkleRoot computes the new Merkle root and proof for an updated user state,
// given the previous proof for that user's state for the previous root.
func UpdateMerkleRoot(prevProof *MProof, newUserState UserState) (*big.Int) {
	// Compute the new leaf hash from the updated user state
	newLeafHash := HashUserState(newUserState)
	currentHash := new(big.Int).Set(newLeafHash)

    // Recompute the root by hashing up the path using the previous proof's siblings
    for i, sibling := range prevProof.Siblings {
        if prevProof.PathBits[i] {
            // Leaf was on the left: hash(currentHash, sibling)
            currentHash = utils.ComputeMiMC(currentHash, sibling)
        } else {
            // Leaf was on the right: hash(sibling, currentHash)
            currentHash = utils.ComputeMiMC(sibling, currentHash)
        }
    }

    return currentHash
}
