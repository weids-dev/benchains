package gateway

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
)

type Player struct {
	ID         int64 `json:"id"`         // ID is the player's unique identifier.
	Balance    int64 `json:"balance"`    // Balance tracks the BEN currency (3 decimal places)
	UsdBalance int64 `json:"usdBalance"` // UsdBalance tracks USD available for exchange
}

// Gateway encapsulates all the resources needed to interact with the Fabric network.
type Gateway struct {
	ClientConnection *grpc.ClientConn
	Gateway          *client.Gateway
	Network          *client.Network
	Contract         *client.Contract
	ChaincodeName    string
	ChannelName      string
}

type Chain struct {
	MspID         string
	CryptoPath    string
	CertPath      string
	KeyPath       string
	TLSCertPath   string
	PeerEndpoint  string
	GatewayPeer   string
	ChannelName   string
	ChaincodeName string
}

// NewGateway initializes a new Gateway instance, similar to what your main() function was doing.
func NewGateway(chain Chain) (*Gateway, error) {
	// 1. Setup gRPC connection
	clientConnection := newGrpcConnection(chain)

	// 2. Create Identity and Sign using the Chain struct
	id, sign, err := newIdentityAndSign(chain)
	if err != nil {
		// Ensure to close the connection if identity creation fails
		clientConnection.Close()
		return nil, err
	}

	// 3. Create a Fabric Gateway instance
	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		client.WithEvaluateTimeout(1*time.Minute),
		client.WithEndorseTimeout(1*time.Minute),
		client.WithSubmitTimeout(1*time.Minute),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		// Make sure to close clientConnection if gateway creation fails
		clientConnection.Close()
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}

	// 4. Get contract
	network := gw.GetNetwork(chain.ChannelName)
	contract := network.GetContract(chain.ChaincodeName)

	return &Gateway{
		ClientConnection: clientConnection,
		Gateway:          gw,
		Network:          network,
		Contract:         contract,
		ChaincodeName:    chain.ChaincodeName,
		ChannelName:      chain.ChannelName,
	}, nil
}

// Close releases all resources held by the Gateway (close gRPC and Gateway).
func (g *Gateway) Close() {
	if g.Gateway != nil {
		g.Gateway.Close()
	}
	if g.ClientConnection != nil {
		g.ClientConnection.Close()
	}
}

// InitLedger invokes the "InitLedger" transaction.
func (g *Gateway) InitLedger() error {
	fmt.Printf("\n--> Submit Transaction: InitLedger\n")

	_, err := g.Contract.SubmitTransaction("InitLedger")
	if err != nil {
		// errorHandling(g.contract, err) // Or define a similar function
		return fmt.Errorf("failed to submit InitLedger transaction: %w", err)
	}

	fmt.Println("*** Transaction committed successfully")
	return nil
}

// GetAllPlayers queries the ledger for all players.
func (g *Gateway) GetAllPlayers() (string, error) {
	fmt.Println("\n--> Evaluate Transaction: GetAllPlayers")

	evaluateResult, err := g.Contract.EvaluateTransaction("GetAllPlayers")
	if err != nil {
		return "", fmt.Errorf("failed to evaluate GetAllPlayers: %w", err)
	}

	result := formatJSON(evaluateResult)
	fmt.Printf("*** Records: %s\n", result)
	return result, nil
}

// GetPlayersNum queries the ledger and prints the number of players.
func (g *Gateway) GetPlayersNum() (int, error) {
	fmt.Println("\n--> Evaluate Transaction: GetAllPlayers to get number of players")

	evaluateResult, err := g.Contract.EvaluateTransaction("GetAllPlayers")
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate transaction: %w", err)
	}

	var players []*Player
	if err := json.Unmarshal(evaluateResult, &players); err != nil {
		return 0, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	fmt.Printf("*** Number of Records: %d\n", len(players))
	return len(players), nil
}

// CreatePlayer submits a transaction to create a new player with a given ID.
func (g *Gateway) CreatePlayer(playerID string) error {
	fmt.Printf("\n--> Submit Transaction: CreatePlayer\n")

	_, err := g.Contract.SubmitTransaction("CreatePlayer", playerID)
	if err != nil {
		return fmt.Errorf("failed to submit CreatePlayer transaction: %w", err)
	}

	fmt.Println("*** Transaction committed successfully")
	return nil
}

// RecordBankTransaction records a new bank transaction on the ledger.
func (g *Gateway) RecordBankTransaction(userID, amountUSDStr, transactionID string) error {
	fmt.Printf("\n--> Submit Transaction: RecordBankTransaction\n")

	_, err := g.Contract.SubmitTransaction("RecordBankTransaction", userID, amountUSDStr, transactionID)
	if err != nil {
		return fmt.Errorf("failed to submit RecordBankTransaction: %w", err)
	}

	fmt.Println("*** Transaction committed successfully")
	return nil
}

// ExchangeInGameCurrency lets users exchange deposited USD to in-game currency.
func (g *Gateway) ExchangeInGameCurrency(userID, transactionID, exchangeRateStr string) error {
	fmt.Printf("\n--> Submit Transaction: ExchangeInGameCurrency\n")

	_, err := g.Contract.SubmitTransaction("ExchangeInGameCurrency", userID, transactionID, exchangeRateStr)
	if err != nil {
		return fmt.Errorf("failed to submit ExchangeInGameCurrency: %w", err)
	}

	fmt.Println("*** Transaction committed successfully")
	return nil
}

// -------------------------------------------------------------
// Helper functions below
// -------------------------------------------------------------
// newGrpcConnection creates a gRPC connection to the Gateway server.
func newGrpcConnection(chain Chain) *grpc.ClientConn {
	certificate, err := loadCertificate(chain.TLSCertPath)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, chain.GatewayPeer)

	connection, err := grpc.Dial(chain.PeerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}

	return connection
}

// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}

// Submit transaction, passing in the wrong number of arguments ,expected to throw an error containing details of any error responses from the smart contract.
func errorHandling(err error) {
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

// newIdentityAndSign creates both identity and signature using the provided Chain configuration.
func newIdentityAndSign(chain Chain) (*identity.X509Identity, identity.Sign, error) {
	certificate, err := loadCertificate(chain.CertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	id, err := identity.NewX509Identity(chain.MspID, certificate)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create identity: %w", err)
	}

	privateKeyPEM, err := loadPrivateKey(chain.KeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load private key: %w", err)
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create signature function: %w", err)
	}

	return id, sign, nil
}

// loadPrivateKey reads the private key from the specified directory.
func loadPrivateKey(keyPath string) ([]byte, error) {
	files, err := os.ReadDir(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key directory: %w", err)
	}

	privateKeyPEM, err := os.ReadFile(path.Join(keyPath, files[0].Name()))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	return privateKeyPEM, nil
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	certificatePEM, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(certificatePEM)
}
