package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"net/http"
	"strings"
	"log"
)

const (
	mspID        = "org01MSP"
	cryptoPath   = "../../networks/fabric/certs/chains/peerOrganizations/org01.chains"
	certPath     = cryptoPath + "/users/User1@org01.chains/msp/signcerts/User1@org01.chains-cert.pem"
	keyPath      = cryptoPath + "/users/User1@org01.chains/msp/keystore/"
	tlsCertPath  = cryptoPath + "/peers/peer1.org01.chains/tls/ca.crt"
	peerEndpoint = "localhost:6001"
	gatewayPeer  = "peer1.org01.chains"
)

// Item represents an in-game item with a name, type, and value.
// It is used to manage the inventory items of a player.
type Item struct {
	Name  string `json:"name"`  // Name is the item's unique identifier.
	Type  string `json:"type"`  // Type categorizes the item.
	Value int    `json:"value"` // Value represents the item's worth or power.
}

// Player represents a game player with a unique ID, balance, and inventory of items.
// It encapsulates the player's state within the game.
type Player struct {
	ID      string  `json:"id"`      // ID is the player's unique identifier.
	Balance float64 `json:"balance"` // Balance tracks the currency the player has.
	Items   []Item  `json:"items"`   // Items hold the collection of items owned by the player.
}

// BankTransaction represents a transaction from the bank to buy in-game currency.
type BankTransaction struct {
	UserID        string  `json:"userID"`
	AmountUSD     float64 `json:"amountUSD"`
	TransactionID string  `json:"transactionID"`
}

var now = time.Now()
var playerId = fmt.Sprintf("player%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)

// TODO: change the rate as the time goes
const rate string = "0.313"

func createPlayerHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		if r.Method == http.MethodGet {
			fmt.Fprintf(w, "Get all players \n")
			getAllPlayers(contract)
			getPlayersNum(contract)
			return
		}
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	playerId := parts[2]

	fmt.Fprintf(w, "Creating player with ID: %s\n", playerId)
	createPlayer(contract, playerId)
	fmt.Fprintf(w, "PUT request processed for playerId: %s", playerId)
}

func depositHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract txID, USD, and playerID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 5 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	transactionId := parts[2]
	USD := parts[3]
	playerId := parts[4]

	fmt.Fprintf(w, "Depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)
	recordBankTransaction(contract, playerId, USD, transactionId)
	fmt.Fprintf(w, "Finish depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)
}

func exchangeHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract txID and playerID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	transactionId := parts[2]
	playerId := parts[3]

	fmt.Fprintf(w, "Exchanging in-game currency for player %s with txID %s\n", playerId, transactionId)
	exchangeInGameCurrency(contract, playerId, transactionId, rate)
	fmt.Fprintf(w, "finish exchanging in-game currency for player %s with txID %s and rate %s\n", playerId, transactionId, rate)
}

func bankExchangeHandler(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	if r.Method != http.MethodPut {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	// Extract txID, USD and playerID from URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 5 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}
	transactionId := parts[2]
	USD := parts[3]
	playerId := parts[4]

	fmt.Fprintf(w, "Depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)
	recordBankTransaction(contract, playerId, USD, transactionId)
	fmt.Fprintf(w, "Finish depositing for player %s with txID %s and amount %s USD\n", playerId, transactionId, USD)

	fmt.Fprintf(w, "Exchanging in-game currency for player %s with txID %s\n", playerId, transactionId)
	exchangeInGameCurrency(contract, playerId, transactionId, rate)
	fmt.Fprintf(w, "finish exchanging in-game currency for player %s with txID %s and rate %s\n", playerId, transactionId, rate)
}

func main() {
	// The gRPC client connection should be shared by all Gateway connections to this endpoint
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	// Create a Gateway connection for a specific client identity
	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		// Default timeouts for different gRPC calls
		client.WithEvaluateTimeout(1*time.Minute),
		client.WithEndorseTimeout(1*time.Minute),
		client.WithSubmitTimeout(1*time.Minute),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer gw.Close()

	// Override default values for chaincode and channel name as they may differ in testing contexts.
	chaincodeName := "basic"
	if ccname := os.Getenv("CHAINCODE_NAME"); ccname != "" {
		chaincodeName = ccname
	}

	channelName := "chains"
	if cname := os.Getenv("CHANNEL_NAME"); cname != "" {
		channelName = cname
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)


	initLedger(contract)
	getAllPlayers(contract)

	http.HandleFunc("/player/", func(w http.ResponseWriter, r *http.Request) {
		createPlayerHandler(w, r, contract)
	})
	http.HandleFunc("/bank/", func(w http.ResponseWriter, r *http.Request) {
		depositHandler(w, r, contract)
	})
	http.HandleFunc("/exchange/", func(w http.ResponseWriter, r *http.Request) {
		exchangeHandler(w, r, contract)
	})
	http.HandleFunc("/bexchange/", func(w http.ResponseWriter, r *http.Request) {
		bankExchangeHandler(w, r, contract)
	})

	fmt.Println("Server is listening on port 10808")

	if err := http.ListenAndServe(":10808", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// newGrpcConnection creates a gRPC connection to the Gateway server.
func newGrpcConnection() *grpc.ClientConn {
	certificate, err := loadCertificate(tlsCertPath)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}

	return connection
}

// newIdentity creates a client identity for this Gateway connection using an X.509 certificate.
func newIdentity() *identity.X509Identity {
	certificate, err := loadCertificate(certPath)
	if err != nil {
		panic(err)
	}

	id, err := identity.NewX509Identity(mspID, certificate)
	if err != nil {
		panic(err)
	}

	return id
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	certificatePEM, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(certificatePEM)
}

// newSign creates a function that generates a digital signature from a message digest using a private key.
func newSign() identity.Sign {
	files, err := os.ReadDir(keyPath)
	if err != nil {
		panic(fmt.Errorf("failed to read private key directory: %w", err))
	}
	privateKeyPEM, err := os.ReadFile(path.Join(keyPath, files[0].Name()))

	if err != nil {
		panic(fmt.Errorf("failed to read private key file: %w", err))
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		panic(err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		panic(err)
	}

	return sign
}

// This type of transaction would typically only be run once by an application the first time it was started after its
// initial deployment. A new version of the chaincode deployed later would likely not need to run an "init" function.
//
// SubmitTransaction will submit a transaction to the ledger and return its result only after it is committed to the ledger.
// The transaction function will be evaluated on endorsing peers and then submitted to the ordering service to be committed to the ledger.
func initLedger(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: InitLedger, function creates the initial set of players on the ledger \n")

	_, err := contract.SubmitTransaction("InitLedger")

	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Evaluate a transaction to query ledger state.
func getAllPlayers(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: GetAllPlayers, function returns all the current players on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("GetAllPlayers")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Records: %s\n", result)
}

// getPlayersNum evaluates a transaction to query ledger state and prints the number of players
func getPlayersNum(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: getPlayersNum, function returns the number of current players on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("GetAllPlayers")
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	// Unmarshal the JSON bytes into a slice of player structs
	var players []*Player
	if err := json.Unmarshal(evaluateResult, &players); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}

	// Now you can accurately get the number of players 
	fmt.Printf("*** Number of Records: %d\n", len(players))
}

// createPlayer directly create a player with all attr initialized default
func createPlayer(contract *client.Contract, playerId string) {
	fmt.Printf("\n--> Submit Transaction: CreatePlayer \n")

	_, err := contract.SubmitTransaction("CreatePlayer", playerId)
	if err != nil {
		errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// recordBankTransaction records a new bank transaction to the ledger
func recordBankTransaction(contract *client.Contract, userID, amountUSDStr, transactionID string) {
	fmt.Printf("\n--> Submit Transaction: RecordBankTransaction \n")

	_, err := contract.SubmitTransaction("RecordBankTransaction", userID, amountUSDStr, transactionID)

	if err != nil {
		// errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// exchangeInGameCurrency let users to exchange their deposited USD to in-game currency
func exchangeInGameCurrency(contract *client.Contract, userID, transactionID, exchangeRateStr string) {
	fmt.Printf("\n--> Submit Transaction: ExchangeInGameCurrency \n")

	_, err := contract.SubmitTransaction("ExchangeInGameCurrency", userID, transactionID, exchangeRateStr)

	if err != nil {
		// errorHandling(contract, err)
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Submit transaction, passing in the wrong number of arguments ,expected to throw an error containing details of any error responses from the smart contract.
func errorHandling(contract *client.Contract, err error) {
	switch err := err.(type) {
	case *client.EndorseError:
		fmt.Printf("Endorse error for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
	case *client.SubmitError:
		fmt.Printf("Submit error for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
	case *client.CommitStatusError:
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Timeout waiting for transaction %s commit status: %s", err.TransactionID, err)
		} else {
			fmt.Printf("Error obtaining commit status for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
		}
	case *client.CommitError:
		fmt.Printf("Transaction %s failed to commit with status %d: %s\n", err.TransactionID, int32(err.Code), err)
	default:
		panic(fmt.Errorf("unexpected error type %T: %w", err, err))
	}

	// Any error that originates from a peer or orderer node external to the gateway will have its details
	// embedded within the gRPC status error. The following code shows how to extract that.
	statusErr := status.Convert(err)

	details := statusErr.Details()
	if len(details) > 0 {
		fmt.Println("Error Details:")

		for _, detail := range details {
			switch detail := detail.(type) {
			case *gateway.ErrorDetail:
				fmt.Printf("- address: %s, mspId: %s, message: %s\n", detail.Address, detail.MspId, detail.Message)
			}
		}
	}
}

// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}
