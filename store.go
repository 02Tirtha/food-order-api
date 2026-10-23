package main

// Package main: provides the in-memory data store for products and orders.

import (
	// errors package is used to create and manipulate error values, such as our sentinel ErrNotFound.
	"errors"
	// sort package provides primitives for sorting slices (e.g. sorting orders by ID).
	"sort"
	// sync package provides synchronization primitives such as sync.RWMutex for concurrent memory access.
	"sync"
)

// ErrNotFound is a sentinel error returned when a requested product or order does not exist in the store.
// Sentinel errors in Go allow callers to check for specific error conditions using errors.Is().
var ErrNotFound = errors.New("resource not found")

// Store holds the in-memory data for products and orders.
// Because an HTTP server in Go processes each incoming request in its own separate goroutine,
// all internal state must be protected against concurrent read and write access.
type Store struct {
	// mu is a Read/Write Mutex that guards all fields below.
	// Multiple goroutines can hold a read lock (RLock) simultaneously,
	// but only one goroutine can hold an exclusive write lock (Lock) at a time.
	mu          sync.RWMutex
	products    map[int]Product
	orders      map[int]Order
	nextOrderID int
}

// NewStore initializes a new Store and seeds it with default products for testing.
// In Go, constructors are typically named New... or New<Type> and return a pointer to the initialized struct.
func NewStore() *Store {
	s := &Store{
		products:    make(map[int]Product),
		orders:      make(map[int]Order),
		nextOrderID: 1,
	}

	// Seed products as specified in the project requirements.
	// Product 4 is marked available=false to enable testing out-of-stock validation.
	seedProducts := []Product{
		{ID: 1, Name: "Veg Burger", PricePaise: 14900, PriceRupees: 149.00, Available: true},
		{ID: 2, Name: "French Fries", PricePaise: 9900, PriceRupees: 99.00, Available: true},
		{ID: 3, Name: "Cold Coffee", PricePaise: 12000, PriceRupees: 120.00, Available: true},
		{ID: 4, Name: "Chocolate Brownie", PricePaise: 8000, PriceRupees: 80.00, Available: false},
	}

	for _, p := range seedProducts {
		s.products[p.ID] = p
	}

	return s
}

// ListProducts returns a copy of all products currently in the store, sorted by ID ascending.
// We return copies rather than pointers so callers cannot mutate the store's internal state.
func (s *Store) ListProducts() []Product {
	// RLock allows concurrent readers without blocking each other.
	s.mu.RLock()
	// defer ensures RUnlock is executed as soon as this function returns, preventing deadlocks.
	defer s.mu.RUnlock()

	// Initialize with make([]Product, 0, len(s.products)) so it serializes to [] in JSON even if empty (never null).
	result := make([]Product, 0, len(s.products))
	for _, p := range s.products {
		result = append(result, p)
	}

	// Sort by ID to ensure deterministic, consistent ordering across requests.
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// GetProduct retrieves a single product by its ID.
// Returns ErrNotFound if the product does not exist.
func (s *Store) GetProduct(id int) (Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.products[id]
	if !exists {
		return Product{}, ErrNotFound
	}

	// Product is a value type with only primitives, so returning 'p' returns a safe copy.
	return p, nil
}

// CreateOrder assigns a unique auto-incrementing ID to the order, records it,
// and returns a safe deep copy of the stored order.
func (s *Store) CreateOrder(order Order) (Order, error) {
	// An exclusive write lock is required because we are mutating s.nextOrderID and s.orders.
	s.mu.Lock()
	defer s.mu.Unlock()

	order.ID = s.nextOrderID
	s.nextOrderID++

	// Deep copy the Items slice so modifying the caller's slice cannot affect the stored order.
	storedOrder := cloneOrder(order)
	s.orders[order.ID] = storedOrder

	// Return another cloned copy to the caller to maintain complete encapsulation.
	return cloneOrder(storedOrder), nil
}

// GetOrder retrieves an order by its ID.
// Returns ErrNotFound if no order exists with that ID.
func (s *Store) GetOrder(id int) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	o, exists := s.orders[id]
	if !exists {
		return Order{}, ErrNotFound
	}

	// Always return a deep copy so callers cannot modify the order's items slice without the lock.
	return cloneOrder(o), nil
}

// ListOrders returns all orders sorted by ID in ascending order.
// Always returns a non-nil slice (empty [] in JSON, never null).
func (s *Store) ListOrders() []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Order, 0, len(s.orders))
	for _, o := range s.orders {
		result = append(result, cloneOrder(o))
	}

	// Sort by ID ascending as required by Module 6.
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

// UpdateOrder updates an existing order in the store.
// Returns ErrNotFound if the order ID is not present.
func (s *Store) UpdateOrder(order Order) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.orders[order.ID]; !exists {
		return Order{}, ErrNotFound
	}

	storedOrder := cloneOrder(order)
	s.orders[order.ID] = storedOrder

	return cloneOrder(storedOrder), nil
}

// cloneOrder creates a deep copy of an Order, ensuring the underlying Items slice is duplicated.
// In Go, slices are reference headers over an underlying array; copying a struct with a slice
// still shares the underlying slice array unless explicitly cloned.
func cloneOrder(o Order) Order {
	cloned := o
	if o.Items != nil {
		cloned.Items = make([]OrderItem, len(o.Items))
		copy(cloned.Items, o.Items)
	}
	return cloned
}
