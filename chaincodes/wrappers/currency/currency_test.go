package currency

import (
	"encoding/json"
	"testing"
	"github.com/stretchr/testify/mock"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-chaincode-go/shim"

        "github.com/weids-dev/benchains/chaincodes/wrappers/types"
)

// MockTransactionContext is a mock of TransactionContextInterface
type MockTransactionContext struct {
	mock.Mock
	contractapi.TransactionContext
}

// MockStub is a mock of ChaincodeStubInterface
type MockStub struct {
	mock.Mock
	shim.ChaincodeStubInterface
}

// PutState mocks the method for putting state into the ledger
func (ms *MockStub) PutState(key string, value []byte) error {
	args := ms.Called(key, value)
	return args.Error(0)
}

// GetState mocks the method for getting state from the ledger
func (ms *MockStub) GetState(key string) ([]byte, error) {
	args := ms.Called(key)
	// handle nil, nil
	result, ok := args.Get(0).([]byte)
	if !ok {
		return nil, args.Error(1)
	}
	return result, args.Error(1)
}

func (m *MockTransactionContext) GetStub() shim.ChaincodeStubInterface {
	args := m.Called()
	return args.Get(0).(*MockStub)
}

// TestInitLedger tests the InitLedger function for success
func TestInitLedger(t *testing.T) {
	ctx := new(MockTransactionContext)
	stub := new(MockStub)
	ctx.On("GetStub").Return(stub)

	// Mock GetState to simulate the player does not exist
	stub.On("GetState", mock.AnythingOfType("string")).Return(nil, nil)
	// Mock PutState to simulate successful write to the ledger ([]byte is an alias for []uint8)
	stub.On("PutState", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).Return(nil)

	cc := new(CurrencyContract)

	// Prepare test data using the updated Player structure from the types package
	testPlayers := []types.Player{
		{
			ID:      "player1",
			Balance: 1000,
		},
		{
			ID:      "player2",
			Balance: 1500,
		},
		{
			ID:      "player3",
			Balance: 500,
		},
	}

	for _, player := range testPlayers {
		playerJSON, _ := json.Marshal(player)
		stub.On("PutState", player.ID, playerJSON).Return(nil)
	}

	err := cc.InitLedger(ctx)
	if err != nil {
		t.Errorf("InitLedger failed with error: %s", err)
	}

	// Assert that PutState was called the correct number of times with the expected arguments
	stub.AssertNumberOfCalls(t, "PutState", len(testPlayers))
	stub.AssertExpectations(t)
}
