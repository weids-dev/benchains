package currency

import (
	"strconv"
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"

	"github.com/weids-dev/benchains/chaincodes/wrappers/types"
)

// CurrencyContract defines the Smart Contract structure.
type CurrencyContract struct {
	contractapi.Contract
}

const PLAYER string = "PLAYER"
const TRANSACTION string = "TRANS"

// InitLedger adds a base set of players to the ledger
func (c *CurrencyContract) InitLedger(ctx contractapi.TransactionContextInterface) error {

	err := c.CreatePlayer(ctx, "player1")
	err = c.CreatePlayer(ctx, "player2")
	err = c.CreatePlayer(ctx, "player3")

	if err != nil {
		return err
	}

	return nil
}

// CreatePlayer adds a new player to the ledger, and initialize it
func (c *CurrencyContract) CreatePlayer(ctx contractapi.TransactionContextInterface, id string) error {
	exists, err := c.PlayerExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the player %s already exists", id)
	}

	player := types.Player{
		ID: id,
		Balance: 0,
	}

	// Marshal Player to JSON
	playerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	// TODO: helper function
	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{player.ID})
	if err != nil {
		return err
	}

	// Store the Player in the ledger
	return ctx.GetStub().PutState(player_key, playerJSON)
}

// PlayerExists returns true if a player with the given ID exists in the ledger
func (c *CurrencyContract) PlayerExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	// TODO: helper function
	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{id})
	if err != nil {
		return false, err
	}

	playerJSON, err := ctx.GetStub().GetState(player_key)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return playerJSON != nil, nil
}

// RecordBankTransaction records a new bank transaction to the ledger.
func (c *CurrencyContract) RecordBankTransaction(ctx contractapi.TransactionContextInterface, userID, amountUSDStr, transactionID string) error {
	amountUSD, err := strconv.ParseFloat(amountUSDStr, 64);

	if err != nil {
		return fmt.Errorf("invalid amountUSD: %s", err)
	}

	transaction := types.BankTransaction{
		UserID:       userID,
		AmountUSD:    amountUSD,
		TransactionID: transactionID,
	}

	transactionJSON, err := json.Marshal(transaction)
	if err != nil {
		return err
	}

	// TODO: helper function
	transaction_key, err := ctx.GetStub().CreateCompositeKey(TRANSACTION, []string{transactionID})
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(transaction_key, transactionJSON)
}

// ExchangeInGameCurrency allows users to exchange their USD to in-game currency.
func (c *CurrencyContract) ExchangeInGameCurrency(ctx contractapi.TransactionContextInterface, userID, transactionID, exchangeRateStr string) error {
	exchangeRate, err := strconv.ParseFloat(exchangeRateStr, 64);

	if err != nil {
		return fmt.Errorf("invalid amountUSD: %s", err)
	}

	// TODO: helper function
	transaction_key, err := ctx.GetStub().CreateCompositeKey(TRANSACTION, []string{transactionID})
	if err != nil {
		return err
	}

	transactionJSON, err := ctx.GetStub().GetState(transaction_key)
	if err != nil {
		return fmt.Errorf("failed to get bank transaction: %v", err)
	}
	if transactionJSON == nil {
		return fmt.Errorf("bank transaction not found")
	}

	var transaction types.BankTransaction
	err = json.Unmarshal(transactionJSON, &transaction)
	if err != nil {
		return err
	}

	if transaction.UserID != userID {
		return fmt.Errorf("user unmatch! please only use the transaction record with your own id to exchange!")
	}

	// TODO: helper function
	player_key, err := ctx.GetStub().CreateCompositeKey(PLAYER, []string{userID})
	if err != nil {
		return err
	}

	// Retrieve the user's current balance.
	playerJSON, err := ctx.GetStub().GetState(player_key)
	if err != nil {
		return fmt.Errorf("failed to get user %s: %v", transaction.UserID, err)
	}
	if playerJSON == nil {
		return fmt.Errorf("user not found")
	}

	var player types.Player
	err = json.Unmarshal(playerJSON, &player)
	if err != nil {
		return err
	}

	// Calculate the equivalent in-game currency and update the player's balance.
	inGameCurrency := transaction.AmountUSD * exchangeRate
	player.Balance += inGameCurrency

	updatedPlayerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	// Update the player's balance in the ledger.
	return ctx.GetStub().PutState(player_key, updatedPlayerJSON)
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
