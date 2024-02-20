/*
   Copyright 2021 IBM All Rights Reserved.
   With Customized Modifications
   Asset Transfer Sample Benchmark
   SPDX-License-Identifier: Apache-2.0
*/

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

// Asset describes basic details of what makes up a simple asset
type Asset struct {
	ID             string `json:"ID"`
	Color          string `json:"color"`
	Size           int    `json:"size"`
	Owner          string `json:"owner"`
	AppraisedValue int    `json:"appraisedValue"`
}

var now = time.Now()
var assetId = fmt.Sprintf("asset%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)

func handleRequest(w http.ResponseWriter, r *http.Request, contract *client.Contract) {
	// Extract assetId from the URL path, assuming the base path is "/asset/"
	assetId := strings.TrimPrefix(r.URL.Path, "/asset/")
	fmt.Printf("assetId is: %s", assetId)
	if assetId == "" {
		http.Error(w, "Asset ID is required", http.StatusBadRequest)
		return
	}

	// Determine the action based on the HTTP method
	switch r.Method {
	case http.MethodPut:
		// Handle PUT request for the specified assetId
		createAsset(contract, assetId)
		fmt.Fprintf(w, "PUT request processed for assetId: %s", assetId)
		getAssetsNum(contract)
	case http.MethodDelete:
		// Handle DELETE request for the specified assetId
		deleteAsset(contract, assetId)
		fmt.Fprintf(w, "DELETE request processed for assetId: %s", assetId)
		getAssetsNum(contract)
	default:
		// Method Not Allowed for other HTTP methods
		getAllAssets(contract)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
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
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
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
	getAllAssets(contract)

	http.HandleFunc("/asset/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, contract)
	})

	fmt.Println("Server is listening on port 10808")

	if err := http.ListenAndServe(":10808", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	/*
	createAsset(contract)
	readAssetByID(contract)
	transferAssetAsync(contract)
	exampleErrorHandling(contract)
	*/
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
	fmt.Printf("\n--> Submit Transaction: InitLedger, function creates the initial set of assets on the ledger \n")

	_, err := contract.SubmitTransaction("InitLedger")

	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Evaluate a transaction to query ledger state.
func getAllAssets(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: GetAllAssets, function returns all the current assets on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Records: %s\n", result)
}

// getAssetsNum evaluates a transaction to query ledger state and prints the number of assets
func getAssetsNum(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: GetAssetsNum, function returns the number of current assets on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}

	// Unmarshal the JSON bytes into a slice of Asset structs
	var assets []*Asset
	if err := json.Unmarshal(evaluateResult, &assets); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}

	// Now you can accurately get the number of assets
	fmt.Printf("*** Number of Records: %d\n", len(assets))
}

// Submit a transaction synchronously, blocking until it has been committed to the ledger.
func createAsset(contract *client.Contract, assetId string) {
	fmt.Printf("\n--> Submit Transaction: CreateAsset, creates new asset with ID, Color, Size, Owner and AppraisedValue arguments \n")

	_, err := contract.SubmitTransaction("CreateAsset", assetId, "yellow", "5", "Tom", "1300")
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Evaluate a transaction by assetID to query ledger state.
func readAssetByID(contract *client.Contract) {
	fmt.Printf("\n--> Evaluate Transaction: ReadAsset, function returns asset attributes\n")

	evaluateResult, err := contract.EvaluateTransaction("ReadAsset", assetId)
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Submit transaction asynchronously, blocking until the transaction has been sent to the orderer, and allowing
// this thread to process the chaincode response (e.g. update a UI) without waiting for the commit notification
func transferAssetAsync(contract *client.Contract) {
	fmt.Printf("\n--> Async Submit Transaction: TransferAsset, updates existing asset owner")

	submitResult, commit, err := contract.SubmitAsync("TransferAsset", client.WithArguments(assetId, "Mark"))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction asynchronously: %w", err))
	}

	fmt.Printf("\n*** Successfully submitted transaction to transfer ownership from %s to Mark. \n", string(submitResult))
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

func deleteAsset(contract *client.Contract, assetId string) {
	fmt.Printf("\n--> Delete Asset: %s \n", assetId)

	_, err := contract.SubmitTransaction("DeleteAsset", assetId)

	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}


// Submit transaction, passing in the wrong number of arguments ,expected to throw an error containing details of any error responses from the smart contract.
func exampleErrorHandling(contract *client.Contract) {
	fmt.Println("\n--> Submit Transaction: UpdateAsset asset70, asset70 does not exist and should return an error")

	_, err := contract.SubmitTransaction("UpdateAsset", "asset70", "blue", "5", "Tomoko", "300")
	if err == nil {
		panic("******** FAILED to return an error")
	}

	fmt.Println("*** Successfully caught the error:")

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
