package utils


import (
	"math/big"

	// ---------------------------
	//  GNARK-CRYPTO libraries
	// ---------------------------
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	gcHash "github.com/consensys/gnark-crypto/hash"
)


//--------------------------------------------------------------------------------
// Helper function to do off-circuit MiMC(BN254) using gnark-crypto
//--------------------------------------------------------------------------------

// ComputeMiMC takes two big.Ints (representing BobBalance, SiblingBalance),
// writes them into a gnark-crypto MiMC_BN254 hasher, and returns the resulting
// big.Int (field element) that matches exactly what in-circuit MiMC(BN254) computes.

func ComputeMiMC(b1, b2 *big.Int) *big.Int {
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
