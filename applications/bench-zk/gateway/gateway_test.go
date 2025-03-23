package gateway

import (
	"fmt"
	"testing"
)

func getDefaultChainConfig() Chain {
	ChainCryptoPath := "../../../networks/fabric/certs/chains/peerOrganizations/org01.chains"
	return Chain{
		MspID:         "org01MSP",
		CryptoPath:    "../../../networks/fabric/certs/chains/peerOrganizations/org01.chains",
		CertPath:      ChainCryptoPath + "/users/User1@org01.chains/msp/signcerts/User1@org01.chains-cert.pem",
		KeyPath:       ChainCryptoPath + "/users/User1@org01.chains/msp/keystore/",
		TLSCertPath:   ChainCryptoPath + "/peers/peer1.org01.chains/tls/ca.crt",
		PeerEndpoint:  "localhost:6001",
		GatewayPeer:   "peer1.org01.chains",
		ChannelName:   "chains",
		ChaincodeName: "basic",
	}
}

var gw *Gateway

// TestMain will set up the gateway for all tests
func TestMain(m *testing.M) {
	chainConfig := getDefaultChainConfig()

	var err error
	gw, err = NewGateway(chainConfig)
	if err != nil {
		return
	}
	defer gw.Close()

	// Run the tests
	m.Run()
}

func TestInit(t *testing.T) {
	if err := gw.InitLedger(); err != nil {
		t.Fatalf("InitLedger failed: %v\n", err)
	}

	allPlayers, err := gw.GetAllPlayers()
	if err != nil {
		t.Fatalf("GetAllPlayers failed: %v\n", err)
	}
	fmt.Println("All Players:", allPlayers)
}

func TestGetPlayersNum(t *testing.T) {
	// Create 5 players
	for i := 4; i <= 8; i++ {
		playerID := fmt.Sprintf("player%d", i)
		if err := gw.CreatePlayer(playerID); err != nil {
			t.Fatalf("CreatePlayer failed for %s: %v\n", playerID, err)
		}
	}
	// Test GetPlayersNum
	num, err := gw.GetPlayersNum()
	if err != nil {
		t.Fatalf("GetPlayersNum failed: %v\n", err)
	}
	fmt.Printf("Number of players: %d\n", num)

	// Expected number of players
	expectedNum := 8
	if num != expectedNum {
		t.Errorf("Expected %d players, got %d", expectedNum, num)
	}
}

func TestRecordBankTransaction(t *testing.T) {
	userID := "player4"
	amountUSDStr := "1000"
	transactionID := "txn001"

	// Call RecordBankTransaction
	err := gw.RecordBankTransaction(userID, amountUSDStr, transactionID)
	if err != nil {
		t.Fatalf("RecordBankTransaction failed: %v\n", err)
	}

	// Optionally, you can verify if the transaction was correctly recorded in the ledger.
	// This might involve querying the ledger to check if the transaction exists, depending on the functionality.
	fmt.Println("Bank transaction recorded successfully.")
	allPlayers, err := gw.GetAllPlayers()
	if err != nil {
		t.Fatalf("GetAllPlayers failed: %v\n", err)
	}
	fmt.Println("All Players:", allPlayers)
}

func TestExchangeInGameCurrency(t *testing.T) {
	userID := "player4"
	transactionID := "txn001"
	exchangeRateStr := "0.313" // Example exchange rate for USD to in-game currency

	// Call ExchangeInGameCurrency
	err := gw.ExchangeInGameCurrency(userID, transactionID, exchangeRateStr)
	if err != nil {
		t.Fatalf("ExchangeInGameCurrency failed: %v\n", err)
	}

	// Optionally, you can verify if the currency exchange was correctly processed.
	// This might involve checking if the user's in-game currency balance has been updated, for example.
	fmt.Println("In-game currency exchange completed successfully.")
	allPlayers, err := gw.GetAllPlayers()
	if err != nil {
		t.Fatalf("GetAllPlayers failed: %v\n", err)
	}
	fmt.Println("All Players:", allPlayers)
}
