// Package types defines the basic structures used throughout the application.
package types

// Player represents a game player with a unique ID, balance, and inventory of items.
// It encapsulates the player's state within the game.
type Player struct {
	ID         int64 `json:"id"`         // ID is the player's unique identifier.
	Balance    int64 `json:"balance"`    // Balance tracks the BEN currency (3 decimal places)
	UsdBalance int64 `json:"usdBalance"` // UsdBalance tracks USD available for exchange
}

// BankTransaction represents a transaction from the bank to buy in-game currency.
type BankTransaction struct {
	UserID        int64 `json:"userID"`
	AmountUSD     int64 `json:"amountUSD"` // Amount in USD (3 decimal places)
	TransactionID int64 `json:"transactionID"`
}
