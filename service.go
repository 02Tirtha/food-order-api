package main

// Package main: provides the business logic layer between HTTP handlers and the data store.

import (
	// errors package is used for sentinel errors.
	"errors"
	// fmt allows formatted string construction, e.g. for error messages.
	"fmt"
	// time package provides time.Now().UTC() for order timestamps.
	"time"
)

// ErrInvalidTransition is a sentinel error returned when a requested order status move is not allowed.
var ErrInvalidTransition = errors.New("invalid status transition")

// ValidationError represents an input or business rule validation failure.
// Handlers inspect this type and translate it into HTTP 400 Bad Request.
type ValidationError struct {
	Message string
}

// Error implements the standard Go 'error' interface for ValidationError.
// In Go, any type that implements the Error() string method satisfies the error interface.
func (e *ValidationError) Error() string {
	return e.Message
}

// newValidationError is a helper function to instantiate a ValidationError pointer.
func newValidationError(msg string) error {
	return &ValidationError{Message: msg}
}

// Service coordinates business rules, input validations, calculations, and lifecycle transitions.
// Decoupling business logic from HTTP transport handlers ensures the core logic is clean,
// reusable, and straightforward to unit-test without running HTTP servers.
type Service struct {
	// store is a pointer to the thread-safe Store holding in-memory data.
	store *Store
}

// NewService constructs a new Service instance backed by the provided Store.
func NewService(store *Store) *Service {
	return &Service{
		store: store,
	}
}

// ListProducts returns all menu products from the store.
func (s *Service) ListProducts() []Product {
	return s.store.ListProducts()
}

// GetProduct retrieves a single product by its ID or returns ErrNotFound.
func (s *Service) GetProduct(id int) (Product, error) {
	return s.store.GetProduct(id)
}

// ListOrders returns all placed orders sorted by ID ascending.
// If no orders exist, it returns an initialized empty slice (never nil), ensuring JSON serializes as [].
func (s *Service) ListOrders() []Order {
	return s.store.ListOrders()
}

// GetOrder retrieves a specific order by its unique ID or returns ErrNotFound.
func (s *Service) GetOrder(id int) (Order, error) {
	return s.store.GetOrder(id)
}

// CreateOrder applies business rules to validate the request, merge items, calculate totals,
// and persist the newly placed order.
func (s *Service) CreateOrder(req CreateOrderRequest) (Order, error) {
	// 1. Validate that the items array is not empty.
	if len(req.Items) == 0 {
		return Order{}, newValidationError("order must contain at least one item")
	}

	// 2. Validate quantities and merge duplicate product IDs while preserving arrival order.
	// We use an ordered slice of IDs along with a map to track combined quantities.
	mergedQuantities := make(map[int]int)
	var orderedProductIDs []int

	for _, item := range req.Items {
		// Rule: quantity must be at least 1.
		if item.Quantity < 1 {
			return Order{}, newValidationError("item quantity must be at least 1")
		}

		if _, seen := mergedQuantities[item.ProductID]; !seen {
			orderedProductIDs = append(orderedProductIDs, item.ProductID)
		}
		mergedQuantities[item.ProductID] += item.Quantity
	}

	// 3. Validate existence and availability of each product, and calculate line totals.
	orderItems := make([]OrderItem, 0, len(orderedProductIDs))
	var totalPaise int64

	for _, productID := range orderedProductIDs {
		// Rule: product must exist in the store (returns ErrNotFound -> 404).
		prod, err := s.store.GetProduct(productID)
		if err != nil {
			return Order{}, err
		}

		// Rule: product must be currently available (returns ValidationError -> 400).
		if !prod.Available {
			return Order{}, newValidationError(fmt.Sprintf("product '%s' is currently unavailable", prod.Name))
		}

		qty := mergedQuantities[productID]
		lineTotal := int64(qty) * prod.PricePaise
		totalPaise += lineTotal

		// Copy name and unit price exclusively from the stored product (ignoring any client prices).
		orderItems = append(orderItems, OrderItem{
			ProductID:       prod.ID,
			Name:            prod.Name,
			Quantity:        qty,
			UnitPricePaise:  prod.PricePaise,
			UnitPriceRupees: prod.PriceRupees,
		})
	}

	// 4. Construct initial order in PLACED status with UTC timestamp.
	newOrder := Order{
		Items:       orderItems,
		Status:      StatusPlaced,
		TotalPaise:  totalPaise,
		TotalRupees: float64(totalPaise) / 100.0,
		CreatedAt:   time.Now().UTC(),
	}

	// 5. Save the order in the store (auto-assigns unique ID).
	return s.store.CreateOrder(newOrder)
}

// IsValidStatus verifies whether an OrderStatus is one of the recognized enum values.
func IsValidStatus(status OrderStatus) bool {
	switch status {
	case StatusPlaced, StatusConfirmed, StatusPreparing, StatusOutForDelivery, StatusDelivered, StatusCancelled:
		return true
	default:
		return false
	}
}

// CanTransition is a pure function (no store access) that checks if moving from 'from' to 'to' is allowed.
// Allowed forward transitions:
// PLACED -> CONFIRMED -> PREPARING -> OUT_FOR_DELIVERY -> DELIVERED
// DELIVERED and CANCELLED are final terminal states.
func CanTransition(from, to OrderStatus) bool {
	switch from {
	case StatusPlaced:
		return to == StatusConfirmed
	case StatusConfirmed:
		return to == StatusPreparing
	case StatusPreparing:
		return to == StatusOutForDelivery
	case StatusOutForDelivery:
		return to == StatusDelivered
	default:
		return false
	}
}

// CanCancel is a pure function that checks if an order can be cancelled.
// Orders may only be cancelled while in PLACED or CONFIRMED status.
func CanCancel(from OrderStatus) bool {
	return from == StatusPlaced || from == StatusConfirmed
}

// UpdateOrderStatus advances an order to its next lifecycle status.
// Rejects unknown statuses and CANCELLED (cancellation has its own endpoint),
// verifies order existence, and enforces valid lifecycle transitions.
func (s *Service) UpdateOrderStatus(id int, newStatus OrderStatus) (Order, error) {
	if !IsValidStatus(newStatus) {
		return Order{}, newValidationError(fmt.Sprintf("unknown status: %s", newStatus))
	}

	if newStatus == StatusCancelled {
		return Order{}, newValidationError("cancellation must be performed via POST /orders/{id}/cancel")
	}

	order, err := s.store.GetOrder(id)
	if err != nil {
		return Order{}, err
	}

	if !CanTransition(order.Status, newStatus) {
		// %w wraps ErrInvalidTransition so errors.Is() detects the conflict sentinel.
		return Order{}, fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidTransition, order.Status, newStatus)
	}

	order.Status = newStatus
	return s.store.UpdateOrder(order)
}

// CancelOrder transitions an order to CANCELLED status if it is currently PLACED or CONFIRMED.
func (s *Service) CancelOrder(id int) (Order, error) {
	order, err := s.store.GetOrder(id)
	if err != nil {
		return Order{}, err
	}

	if !CanCancel(order.Status) {
		return Order{}, fmt.Errorf("%w: cannot cancel order in %s status", ErrInvalidTransition, order.Status)
	}

	order.Status = StatusCancelled
	return s.store.UpdateOrder(order)
}

