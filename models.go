package main

// Package main: all our project files are part of the 'main' package, forming a single executable.

import (
	// time package provides the time.Time struct used for tracking order creation timestamps.
	"time"
)

// OrderStatus defines a distinct custom type based on Go's primitive 'string'.
// Using a custom type rather than a raw string gives us type safety,
// preventing accidental assignment of arbitrary strings to an order's status.
type OrderStatus string

// Enum-like constants for order lifecycle statuses.
// In Go, constants are declared using the 'const' block.
// Statuses represent the strict progression an order can take from placement to completion.
const (
	StatusPlaced         OrderStatus = "PLACED"
	StatusConfirmed      OrderStatus = "CONFIRMED"
	StatusPreparing      OrderStatus = "PREPARING"
	StatusOutForDelivery OrderStatus = "OUT_FOR_DELIVERY"
	StatusDelivered      OrderStatus = "DELIVERED"
	StatusCancelled      OrderStatus = "CANCELLED"
)

// Product represents a menu item available in the food delivery system.
// Struct tags (e.g., `json:"id"`) instruct the encoding/json package to map
// exported Go struct fields (which must be Capitalized) to lowercase snake_case JSON keys.
type Product struct {
	// ID is the unique integer identifier for the product.
	ID int `json:"id"`
	// Name is the display title of the dish or drink.
	Name string `json:"name"`
	// PricePaise stores money as an integer in the smallest currency unit (Paise).
	// We never use floating-point types for internal money calculations to avoid precision bugs.
	PricePaise int64 `json:"price_paise"`
	// PriceRupees is a convenience helper field showing the user-friendly real-world price in Rupees (e.g. 149 for Rs 149.00).
	PriceRupees float64 `json:"price_rupees"`
	// Available flags whether the kitchen is currently serving this item.
	Available bool `json:"available"`
}

// OrderItem represents a single line item within a finalized customer order.
// It snapshots the product details (name and unit price at the time of purchase)
// so that future menu price changes do not alter historical orders.
type OrderItem struct {
	ProductID        int     `json:"product_id"`
	Name             string  `json:"name"`
	Quantity         int     `json:"quantity"`
	UnitPricePaise   int64   `json:"unit_price_paise"`
	UnitPriceRupees  float64 `json:"unit_price_rupees"`
}

// Order represents a complete customer order.
type Order struct {
	ID          int         `json:"id"`
	Items       []OrderItem `json:"items"`
	Status      OrderStatus `json:"status"`
	TotalPaise  int64       `json:"total_paise"`
	TotalRupees float64     `json:"total_rupees"`
	CreatedAt   time.Time   `json:"created_at"`
}

// CreateOrderItemRequest represents a single item in the incoming POST /orders payload.
// Note that clients only provide product_id and quantity; the server calculates prices.
type CreateOrderItemRequest struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// CreateOrderRequest defines the incoming request payload structure for placing an order.
// Unknown or unexpected JSON fields will be rejected during JSON decoding.
type CreateOrderRequest struct {
	Items []CreateOrderItemRequest `json:"items"`
}

// UpdateStatusRequest defines the incoming request payload for updating an order's status via PATCH.
type UpdateStatusRequest struct {
	Status OrderStatus `json:"status"`
}

// ErrorResponse defines the standard JSON error shape required by the API specification:
// {"error": "human readable message"}
type ErrorResponse struct {
	Error string `json:"error"`
}
