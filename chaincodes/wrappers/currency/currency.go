package currency

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	"github.com/weids-dev/benchains/chaincodes/wrappers/types"
)

// CurrencyContract defines the Smart Contract structure.
type CurrencyContract struct {
	contractapi.Contract
	ExchangeRate int64 `json:"exchangeRate"` // Exchange rate for USD to BEN conversion (3 decimal places)
}

const PLAYER string = "PLAYER"
const TRANSACTION string = "TRANS"

// InitLedger adds a base set of players to the ledger
func (c *CurrencyContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	// Set default exchange rate (1.0 with 3 decimal places = 1000)
	c.ExchangeRate = 1000

	for i := 1; i <= 3; i++ {
		err := c.CreatePlayer(ctx, int64(i))
		if err != nil {
			return err
		}
	}
	return nil
}

// CreatePlayer adds a new player to the ledger, and initialize it
func (c *CurrencyContract) CreatePlayer(ctx contractapi.TransactionContextInterface, id int64) error {
	exists, err := c.PlayerExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the player %d already exists", id)
	}

	player := types.Player{
		ID:         id,
		Balance:    0,
		UsdBalance: 0,
	}

	// Marshal Player to JSON
	playerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{fmt.Sprintf("%d", id)})
	if err != nil {
		return err
	}

	// Store the Player in the ledger
	return ctx.GetStub().PutState(player_key, playerJSON)
}

// PlayerExists returns true if a player with the given ID exists in the ledger
func (c *CurrencyContract) PlayerExists(ctx contractapi.TransactionContextInterface, id int64) (bool, error) {
	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{fmt.Sprintf("%d", id)})
	if err != nil {
		return false, err
	}

	playerJSON, err := ctx.GetStub().GetState(player_key)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return playerJSON != nil, nil
}

// GetPlayer retrieves a player from the ledger
func (c *CurrencyContract) GetPlayer(ctx contractapi.TransactionContextInterface, id int64) (*types.Player, error) {
	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{fmt.Sprintf("%d", id)})
	if err != nil {
		return nil, err
	}

	playerJSON, err := ctx.GetStub().GetState(player_key)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if playerJSON == nil {
		return nil, fmt.Errorf("player %d does not exist", id)
	}

	var player types.Player
	err = json.Unmarshal(playerJSON, &player)
	if err != nil {
		return nil, err
	}

	return &player, nil
}

// RecordBankTransaction records a new bank transaction to the ledger.
func (c *CurrencyContract) RecordBankTransaction(ctx contractapi.TransactionContextInterface, userID, amountUSD, transactionID int64) error {
	// Validate transaction (in a real system, this would verify the bank transaction)
	fmt.Printf("Validating bank transaction ID: %d for user: %d with amount: %d\n", transactionID, userID, amountUSD)

	// Check if player exists
	exists, err := c.PlayerExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("player %d does not exist", userID)
	}

	transaction := types.BankTransaction{
		UserID:        userID,
		AmountUSD:     amountUSD,
		TransactionID: transactionID,
	}

	transactionJSON, err := json.Marshal(transaction)
	if err != nil {
		return err
	}

	transaction_key, err := ctx.GetStub().CreateCompositeKey(TRANSACTION, []string{fmt.Sprintf("%d", transactionID)})
	if err != nil {
		return err
	}

	// Store the transaction
	err = ctx.GetStub().PutState(transaction_key, transactionJSON)
	if err != nil {
		return err
	}

	// Update the player's USD balance
	player, err := c.GetPlayer(ctx, userID)
	if err != nil {
		return err
	}

	// Increase USD balance
	player.UsdBalance += amountUSD

	updatedPlayerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{fmt.Sprintf("%d", userID)})
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(player_key, updatedPlayerJSON)
}

// ExchangeInGameCurrency allows users to exchange currency (USD to BEN or BEN to USD).
func (c *CurrencyContract) ExchangeInGameCurrency(ctx contractapi.TransactionContextInterface, userID, benAmountChange int64) error {
	fmt.Printf("Starting ExchangeInGameCurrency: userID=%d, benAmountChange=%d\n", userID, benAmountChange)

	// Check exchange rate
	if c.ExchangeRate == 0 {
		return fmt.Errorf("exchange rate is zero")
	}
	fmt.Printf("ExchangeRate=%d\n", c.ExchangeRate)

	// Get player
	player, err := c.GetPlayer(ctx, userID)
	if err != nil {
		return err
	}
	fmt.Printf("Player fetched: UsdBalance=%d, Balance=%d\n", player.UsdBalance, player.Balance)

	if benAmountChange > 0 {
		usdRequired := (benAmountChange * 1000) / c.ExchangeRate
		fmt.Printf("usdRequired=%d\n", usdRequired)

		if player.UsdBalance < usdRequired {
			return fmt.Errorf("insufficient USD balance: have %d, need %d", player.UsdBalance, usdRequired)
		}

		player.UsdBalance -= usdRequired
		player.Balance += benAmountChange
		fmt.Printf("Updated: UsdBalance=%d, Balance=%d\n", player.UsdBalance, player.Balance)
	} else {
		benToExchange := -benAmountChange
		if player.Balance < benToExchange {
			return fmt.Errorf("insufficient BEN balance: have %d, need %d", player.Balance, benToExchange)
		}
		usdToAdd := (benToExchange * c.ExchangeRate) / 1000
		player.UsdBalance += usdToAdd
		player.Balance -= benToExchange
	}

	updatedPlayerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{fmt.Sprintf("%d", userID)})
	if err != nil {
		return err
	}

	fmt.Printf("Writing player state: userID=%d\n", userID)
	return ctx.GetStub().PutState(player_key, updatedPlayerJSON)
}

// SetExchangeRate sets the exchange rate for USD to BEN conversion
func (c *CurrencyContract) SetExchangeRate(ctx contractapi.TransactionContextInterface, newRate int64) error {
	c.ExchangeRate = newRate
	return nil
}

func (c *CurrencyContract) GetAllPlayers(ctx contractapi.TransactionContextInterface) ([]*types.Player, error) {
	playerIterator, err := ctx.GetStub().GetStateByPartialCompositeKey(PLAYER, []string{})

	if err != nil {
		return nil, fmt.Errorf("failed to get state by partial composite key: %v", err)
	}

	defer playerIterator.Close()
	var players []*types.Player
	for playerIterator.HasNext() {
		response, err := playerIterator.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over players: %v", err)
		}

		var player types.Player
		err = json.Unmarshal(response.Value, &player)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal player: %v", err)
		}
		players = append(players, &player)
	}

	return players, nil
}
