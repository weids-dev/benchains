package currency

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// CurrencyContract defines the Smart Contract structure.
type CurrencyContract struct {
	contractapi.Contract
}

// Player represents the structure of a player account.
// Adapting to changing needs
type Player struct {
	ID        string                 `json:"id"`
	Attributes map[string]interface{} `json:"attributes"`
}


// InitLedger adds a base set of players to the ledger
func (c *CurrencyContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	players := []struct{
		ID      string
		Balance int
		Items   []string
	}{
		{ID: "player1", Balance: 1000, Items: []string{}},
		{ID: "player2", Balance: 1500, Items: []string{}},
		{ID: "player3", Balance: 500, Items: []string{}},
	}

	for _, player := range players {
		err := c.CreatePlayer(ctx, player.ID, player.Balance, player.Items)
		if err != nil {
			return err
		}
	}

	return nil
}


// CreatePlayer adds a new player to the ledger
func (c *CurrencyContract) CreatePlayer(ctx contractapi.TransactionContextInterface, id string, balance int, items []string) error {
	exists, err := c.PlayerExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the player %s already exists", id)
	}

	// Create a new Player struct, then Marshal to JSON
	player := Player{
		ID: id,
		Attributes: map[string]interface{}{
			"balance": balance,
			"items":   items,
		},
	}
	playerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	// Store the Player in the ledger
	return ctx.GetStub().PutState(id, playerJSON)
}

// PlayerExists returns true if a player with the given ID exists in the ledger
func (c *CurrencyContract) PlayerExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	playerJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return playerJSON != nil, nil
}
