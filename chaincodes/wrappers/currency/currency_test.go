package currency

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-protos-go/ledger/queryresult"
	"github.com/stretchr/testify/mock"

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

// CreateCompositeKey mocks the method for creating a composite key
func (ms *MockStub) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	args := ms.Called(objectType, attributes)
	return args.String(0), args.Error(1)
}

// MockStateIterator is a mock of StateQueryIteratorInterface
type MockStateIterator struct {
	mock.Mock
	shim.StateQueryIteratorInterface
	Results []*queryresult.KV
	Index   int
}

// HasNext mocks the HasNext method
func (msi *MockStateIterator) HasNext() bool {
	return msi.Index < len(msi.Results)
}

// Next mocks the Next method
func (msi *MockStateIterator) Next() (*queryresult.KV, error) {
	if msi.Index < len(msi.Results) {
		result := msi.Results[msi.Index]
		msi.Index++
		return result, nil
	}
	return nil, fmt.Errorf("no more results")
}

// Close mocks the Close method
func (msi *MockStateIterator) Close() error {
	return nil
}

// GetStateByPartialCompositeKey mocks the method for getting state by partial composite key
func (ms *MockStub) GetStateByPartialCompositeKey(objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	args := ms.Called(objectType, keys)
	return args.Get(0).(shim.StateQueryIteratorInterface), args.Error(1)
}

// TestInitLedger tests the InitLedger function for success
func TestInitLedger(t *testing.T) {
	ctx := new(MockTransactionContext)
	stub := new(MockStub)
	ctx.On("GetStub").Return(stub)

	// Mock CreateCompositeKey
	for i := 1; i <= 3; i++ {
		playerID := int64(i)
		compositeKey := "PLAYER_" + fmt.Sprintf("%d", playerID)
		stub.On("CreateCompositeKey", PLAYER, []string{fmt.Sprintf("%d", playerID)}).Return(compositeKey, nil)

		// For PlayerExists check
		stub.On("GetState", compositeKey).Return(nil, nil)

		// For player creation - note we're using the composite key, not just the ID
		stub.On("PutState", compositeKey, mock.AnythingOfType("[]uint8")).Return(nil)
	}

	cc := new(CurrencyContract)
	cc.ExchangeRate = 1000

	err := cc.InitLedger(ctx)
	if err != nil {
		t.Errorf("InitLedger failed with error: %s", err)
	}

	stub.AssertExpectations(t)
}

// TestCreatePlayer tests the CreatePlayer function
func TestCreatePlayer(t *testing.T) {
	ctx := new(MockTransactionContext)
	stub := new(MockStub)
	ctx.On("GetStub").Return(stub)

	cc := new(CurrencyContract)

	playerID := int64(123)
	compositeKey := "PLAYER_" + fmt.Sprintf("%d", playerID)

	// Mock CreateCompositeKey
	stub.On("CreateCompositeKey", PLAYER, []string{fmt.Sprintf("%d", playerID)}).Return(compositeKey, nil)

	// Mock GetState to simulate the player does not exist
	stub.On("GetState", compositeKey).Return(nil, nil)

	// Mock PutState to simulate successful write to the ledger
	player := types.Player{
		ID:         playerID,
		Balance:    0,
		UsdBalance: 0,
	}
	playerJSON, _ := json.Marshal(player)
	stub.On("PutState", compositeKey, playerJSON).Return(nil)

	err := cc.CreatePlayer(ctx, playerID)
	if err != nil {
		t.Errorf("CreatePlayer failed with error: %s", err)
	}

	// Assert that PutState was called once with the expected arguments
	stub.AssertNumberOfCalls(t, "PutState", 1)
	stub.AssertExpectations(t)
}

// TestRecordBankTransaction tests the RecordBankTransaction function
func TestRecordBankTransaction(t *testing.T) {
	ctx := new(MockTransactionContext)
	stub := new(MockStub)
	ctx.On("GetStub").Return(stub)

	cc := new(CurrencyContract)

	userID := int64(123)
	amountUSD := int64(5000) // 5.000 USD
	transactionID := int64(9876)

	// Composite keys
	playerKey := "PLAYER_" + fmt.Sprintf("%d", userID)
	transKey := "TRANS_" + fmt.Sprintf("%d", transactionID)

	// Mock CreateCompositeKey for PlayerExists check
	stub.On("CreateCompositeKey", PLAYER, []string{fmt.Sprintf("%d", userID)}).Return(playerKey, nil)

	// Mock CreateCompositeKey for transaction
	stub.On("CreateCompositeKey", TRANSACTION, []string{fmt.Sprintf("%d", transactionID)}).Return(transKey, nil)

	// Mock player data to be retrieved
	existingPlayer := types.Player{
		ID:         userID,
		Balance:    1000, // 1.000 BEN
		UsdBalance: 2000, // 2.000 USD
	}
	existingPlayerJSON, _ := json.Marshal(existingPlayer)

	// Expected updated player data after transaction
	updatedPlayer := types.Player{
		ID:         userID,
		Balance:    1000, // 1.000 BEN (unchanged)
		UsdBalance: 7000, // 7.000 USD (2.000 + 5.000)
	}
	updatedPlayerJSON, _ := json.Marshal(updatedPlayer)

	// Mock GetState to simulate the player exists
	stub.On("GetState", playerKey).Return(existingPlayerJSON, nil)

	// Mock transaction PutState
	stub.On("PutState", transKey, mock.AnythingOfType("[]uint8")).Return(nil)

	// Mock player PutState
	stub.On("PutState", playerKey, updatedPlayerJSON).Return(nil)

	err := cc.RecordBankTransaction(ctx, userID, amountUSD, transactionID)
	if err != nil {
		t.Errorf("RecordBankTransaction failed with error: %s", err)
	}

	stub.AssertExpectations(t)
}

// TestExchangeInGameCurrency tests the ExchangeInGameCurrency function
func TestExchangeInGameCurrency(t *testing.T) {
	ctx := new(MockTransactionContext)
	stub := new(MockStub)
	ctx.On("GetStub").Return(stub)

	cc := new(CurrencyContract)

	userID := int64(123)
	benAmountChange := int64(2000) // Want to get 2.000 BEN

	// Create the composite key - this is what's missing
	playerKey := "PLAYER_" + fmt.Sprintf("%d", userID)
	stub.On("CreateCompositeKey", PLAYER, []string{fmt.Sprintf("%d", userID)}).Return(playerKey, nil)

	// Mock player data to be retrieved (has enough USD)
	existingPlayer := types.Player{
		ID:         userID,
		Balance:    1000, // 1.000 BEN
		UsdBalance: 5000, // 5.000 USD
	}
	existingPlayerJSON, _ := json.Marshal(existingPlayer)

	// Expected updated player data after exchange
	// With rate 1.000, to get 2.000 BEN requires 2.000 USD
	updatedPlayer := types.Player{
		ID:         userID,
		Balance:    3000, // 3.000 BEN (1.000 + 2.000)
		UsdBalance: 3000, // 3.000 USD (5.000 - 2.000)
	}
	updatedPlayerJSON, _ := json.Marshal(updatedPlayer)

	// Mock GetState to simulate the player exists
	stub.On("GetState", playerKey).Return(existingPlayerJSON, nil)

	// Mock PutState to simulate successful write to the ledger
	stub.On("PutState", playerKey, updatedPlayerJSON).Return(nil)

	cc.ExchangeRate = 1000 // 1.000 exchange rate

	err := cc.ExchangeInGameCurrency(ctx, userID, benAmountChange)
	if err != nil {
		t.Errorf("ExchangeInGameCurrency failed with error: %s", err)
	}

	// Assert that PutState was called once with the expected arguments
	stub.AssertNumberOfCalls(t, "PutState", 1)
	stub.AssertExpectations(t)
}

// TestGetAllPlayers tests the GetAllPlayers function
func TestGetAllPlayers(t *testing.T) {
	ctx := new(MockTransactionContext)
	stub := new(MockStub)
	ctx.On("GetStub").Return(stub)

	cc := new(CurrencyContract)

	// Create sample player data
	players := []*types.Player{
		{ID: 10, Balance: 1000, UsdBalance: 5000},
		{ID: 11, Balance: 2000, UsdBalance: 7000},
		{ID: 12, Balance: 3000, UsdBalance: 9000},
	}

	// Create KV results for the iterator
	var results []*queryresult.KV
	for _, player := range players {
		// Create composite key for each player
		playerKey := "PLAYER_" + fmt.Sprintf("%d", player.ID)
		playerJSON, _ := json.Marshal(player)

		// Create KV pair
		kv := &queryresult.KV{
			Key:   playerKey,
			Value: playerJSON,
		}
		results = append(results, kv)
	}

	// Create and configure the iterator
	iterator := &MockStateIterator{
		Results: results,
		Index:   0,
	}

	// Mock GetStateByPartialCompositeKey to return our iterator
	stub.On("GetStateByPartialCompositeKey", PLAYER, []string{}).Return(iterator, nil)

	// Call GetAllPlayers
	returnedPlayers, err := cc.GetAllPlayers(ctx)
	if err != nil {
		t.Errorf("GetAllPlayers failed with error: %s", err)
	}

	// Verify the results
	if len(returnedPlayers) != len(players) {
		t.Errorf("Expected %d players, got %d", len(players), len(returnedPlayers))
	}

	// Compare each player
	for i, player := range players {
		t.Logf("Player %d: Expected ID %d, got %d", i, player.ID, returnedPlayers[i].ID)
		if returnedPlayers[i].ID != player.ID {
			t.Errorf("Player %d: Expected ID %d, got %d", i, player.ID, returnedPlayers[i].ID)
		}
		if returnedPlayers[i].Balance != player.Balance {
			t.Errorf("Player %d: Expected Balance %d, got %d", i, player.Balance, returnedPlayers[i].Balance)
		}
		if returnedPlayers[i].UsdBalance != player.UsdBalance {
			t.Errorf("Player %d: Expected UsdBalance %d, got %d", i, player.UsdBalance, returnedPlayers[i].UsdBalance)
		}
	}

	stub.AssertExpectations(t)
}
