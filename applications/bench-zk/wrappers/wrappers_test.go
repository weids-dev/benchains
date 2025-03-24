// wrappers/wrappers_test.go
package wrappers

import (
	"context"
	"log"
	"testing"
	"time"

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

	if wp.Gw1 == nil {
		log.Fatalf("Gw1 is not initialized")
	}

	// Check if Gw2 is initialized
	if wp.Gw2 == nil {
		log.Fatalf("Gw2 is not initialized")
	}

	// Check if UserStates is empty
	if len(wp.UserStates) != 0 {
		log.Fatalf("Expected UserStates to be empty, got %d", len(wp.UserStates))
	} else {
		log.Printf("UserStates initialized as empty")
	}

	if err := wp.Gw2.InitLedger(); err != nil {
		log.Fatalf("InitLedger failed: %v\n", err)
	}

	evaluateResult, err := wp.Gw2.Contract.EvaluateTransaction("CurrencyContract:GetAllPlayers")
	if err != nil {
		log.Fatalf("GetAllPlayers failed: %v\n", err)
	}
	log.Printf("EvaluateResult Players: %v", evaluateResult)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run Operate in a goroutine
	go func() {
		if err := wp.Operate(ctx); err != nil {
			log.Fatalf("Operate failed: %v", err)
		}
	}()

	// Wait for 10 seconds to ensure Operate is running
	time.Sleep(10 * time.Second)

	defer wp.Close()
	m.Run()

	<-ctx.Done()
}

// TestSimulateTransactions tests the operator by simulating transactions and observing the output.
func TestSimulateTransactions(t *testing.T) {
	// Wait briefly to ensure Operate starts
	time.Sleep(1 * time.Second)

	// Simulate transactions on Layer 2
	contract := wp.Gw2.Contract

	// Create a player with ID 1
	createPlayer(contract, "4")

	// Record a bank transaction: deposit 100 USD for user 4 (txID 123)
	recordBankTransaction(contract, "4", "100", "123")

	// Exchange 50 USD to BEN (this adds 50 BEN to the user's balance)
	exchangeInGameCurrency(contract, "4", "50.0")

	// Exchange BEN back to USD (this removes 20 BEN from the user's balance)
	exchangeInGameCurrency(contract, "4", "-20.0")

	// Wait for the context to timeout, giving Operate time to process blocks
}

// TestExchangeRateChanges tests the effect of changing the exchange rate.
func TestExchangeRateChanges(t *testing.T) {
	// Wait briefly to ensure Operate starts
	time.Sleep(1 * time.Second)

	contract := wp.Gw2.Contract

	// Create a player
	createPlayer(contract, "5")

	// Deposit 1000 USD
	recordBankTransaction(contract, "5", "1000", "456")

	// Exchange 100 BEN at the default rate (1.0)
	exchangeInGameCurrency(contract, "5", "100.0")

	// Set a new exchange rate (2.0 - meaning 1 USD = 2 BEN)
	_, err := contract.SubmitTransaction("CurrencyContract:SetExchangeRate", "2000") // 2.0 with 3 decimal places
	if err != nil {
		t.Errorf("Failed to set exchange rate: %v", err)
	}

	// Exchange another 100 BEN at the new rate (should cost less USD)
	exchangeInGameCurrency(contract, "5", "100.0")
}
