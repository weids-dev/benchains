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
}

// InitLedger adds a base set of players to the ledger
func (c *CurrencyContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	players := []types.Player{
		{ID: "player1", Balance: 1000, Items: []types.Item{}},
		{ID: "player2", Balance: 1500, Items: []types.Item{}},
		{ID: "player3", Balance: 500, Items: []types.Item{}},
	}

	for _, player := range players {
		err := c.CreatePlayer(ctx, player)
		if err != nil {
			return err
		}
	}

	return nil
}

// CreatePlayer adds a new player to the ledger
func (c *CurrencyContract) CreatePlayer(ctx contractapi.TransactionContextInterface, player types.Player) error {
	exists, err := c.PlayerExists(ctx, player.ID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the player %s already exists", player.ID)
	}

	// Marshal Player to JSON
	playerJSON, err := json.Marshal(player)
	if err != nil {
		return err
	}

	// Store the Player in the ledger
	return ctx.GetStub().PutState(player.ID, playerJSON)
}

// PlayerExists returns true if a player with the given ID exists in the ledger
func (c *CurrencyContract) PlayerExists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	playerJSON, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}
	return playerJSON != nil, nil
}
