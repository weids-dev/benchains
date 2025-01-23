// Package types defines the basic structures used throughout the application.
package types

// Player represents a game player with a unique ID, balance, and inventory of items.
// It encapsulates the player's state within the game.
type Player struct {
	ID      string  `json:"id"`      // ID is the player's unique identifier.
	Balance float64 `json:"balance"` // Balance tracks the currency the player has.
}

// BankTransaction represents a transaction from the bank to buy in-game currency.
type BankTransaction struct {
	UserID        string  `json:"userID"`
	AmountUSD     float64 `json:"amountUSD"`
	TransactionID string  `json:"transactionID"`
}
