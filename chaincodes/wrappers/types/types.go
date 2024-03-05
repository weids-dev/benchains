// Package types defines the basic structures used throughout the application.
package types

// Item represents an in-game item with a name, type, and value.
// It is used to manage the inventory items of a player.
type Item struct {
	Name  string `json:"name"`  // Name is the item's unique identifier.
	Type  string `json:"type"`  // Type categorizes the item.
	Value int    `json:"value"` // Value represents the item's worth or power.
}

// Player represents a game player with a unique ID, balance, and inventory of items.
// It encapsulates the player's state within the game.
type Player struct {
	ID      string `json:"id"`      // ID is the player's unique identifier.
	Balance int    `json:"balance"` // Balance tracks the currency the player has.
	Items   []Item `json:"items"`   // Items hold the collection of items owned by the player.
}
