// wrappers/wrappers_test.go
package wrappers


import (
	"fmt"
	"testing"

	"bench-zk/gateway"
)

// getTestChainConfig1 returns a Chain configuration for testing Layer 1
func getTestChainConfig1() gateway.Chain {
	return gateway.Chain{
		MspID:         "org01MSP",
		CryptoPath:    "../../../networks/fabric/certs/chains/peerOrganizations/org01.chains",
		CertPath:      "../../../networks/fabric/certs/chains/peerOrganizations/org01.chains/users/User1@org01.chains/msp/signcerts/User1@org01.chains-cert.pem",
		KeyPath:       "../../../networks/fabric/certs/chains/peerOrganizations/org01.chains/users/User1@org01.chains/msp/keystore/",
		TLSCertPath:   "../../../networks/fabric/certs/chains/peerOrganizations/org01.chains/peers/peer1.org01.chains/tls/ca.crt",
		PeerEndpoint:  "localhost:6001",
		GatewayPeer:   "peer1.org01.chains",
		ChannelName:   "chains",
		ChaincodeName: "basic",
	}
}

// getTestChainConfig2 returns a Chain configuration for testing Layer 2
func getTestChainConfig2() gateway.Chain {
	return gateway.Chain{
		MspID:         "org02MSP",
		CryptoPath:    "../../../networks/fabric/certs/chains/peerOrganizations/org02.chains",
		CertPath:      "../../../networks/fabric/certs/chains/peerOrganizations/org02.chains/users/User1@org02.chains/msp/signcerts/User1@org02.chains-cert.pem",
		KeyPath:       "../../../networks/fabric/certs/chains/peerOrganizations/org02.chains/users/User1@org02.chains/msp/keystore/",
		TLSCertPath:   "../../../networks/fabric/certs/chains/peerOrganizations/org02.chains/peers/peer1.org02.chains/tls/ca.crt",
		PeerEndpoint:  "localhost:6002",
		GatewayPeer:   "peer1.org02.chains",
		ChannelName:   "chains02",
		ChaincodeName: "pasic",
	}
}

var wp *Wrappers

// TestMain will set up the wrappers for all tests
func TestMain(m *testing.M) {
	// Prepare Chain configurations
	chain1 := getTestChainConfig1()
	chain2 := getTestChainConfig2()

	// Initialize Wrappers
	var err error
	wp, err = NewWrappers(chain1, chain2)
	if err != nil {
		return
	}
	defer wp.Close()

	m.Run()
}

func TestInit(t *testing.T) {
	// Check if Gw1 is initialized
	if wp.Gw1 == nil {
		t.Errorf("Gw1 is not initialized")
	}

	// Check if Gw2 is initialized
	if wp.Gw2 == nil {
		t.Errorf("Gw2 is not initialized")
	}

	// Check if UserStates is empty
	if len(wp.UserStates) != 0 {
		t.Errorf("Expected UserStates to be empty, got %d", len(wp.UserStates))
	} else {
		t.Logf("UserStates initialized as empty")
	}

	// Check if Deposits is empty
	if len(wp.Deposits) != 0 {
		t.Errorf("Expected Deposits to be empty, got %d", len(wp.Deposits))
	} else {
		t.Logf("Deposits initialized as empty")
	}

	if err := wp.Gw2.InitLedger(); err != nil {
		t.Fatalf("InitLedger failed: %v\n", err)
	}

	allPlayers, err := wp.Gw2.GetAllPlayers()
	if err != nil {
		t.Fatalf("GetAllPlayers failed: %v\n", err)
	}
	fmt.Println("All Players:", allPlayers)
}

func TestOperate(t *testing.T) {
	wp.Operate()
}
