package main

// Package main: Go test files in the same directory and package can access all types
// defined across models.go, store.go, service.go, etc.

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestModels_JSONMarshaling validates that all model structs marshal to valid JSON
// with snake_case field names and the exact expected structure required by the specification.
func TestModels_JSONMarshaling(t *testing.T) {
	// 1. Verify Product marshaling
	prod := Product{
		ID:          1,
		Name:        "Veg Burger",
		PricePaise:  14900,
		PriceRupees: 149.00,
		Available:   true,
	}
	prodData, err := json.Marshal(prod)
	if err != nil {
		t.Fatalf("failed to marshal Product: %v", err)
	}
	expectedProdKeys := []string{`"id":1`, `"name":"Veg Burger"`, `"price_paise":14900`, `"price_rupees":149`, `"available":true`}
	for _, key := range expectedProdKeys {
		if !strings.Contains(string(prodData), key) {
			t.Errorf("Product JSON missing key snippet %s, got: %s", key, string(prodData))
		}
	}

	// 2. Verify OrderItem and Order marshaling
	fixedTime := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	order := Order{
		ID: 101,
		Items: []OrderItem{
			{
				ProductID:       1,
				Name:            "Veg Burger",
				Quantity:        2,
				UnitPricePaise:  14900,
				UnitPriceRupees: 149.00,
			},
		},
		Status:      StatusPlaced,
		TotalPaise:  29800,
		TotalRupees: 298.00,
		CreatedAt:   fixedTime,
	}

	orderData, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("failed to marshal Order: %v", err)
	}
	expectedOrderKeys := []string{
		`"id":101`,
		`"items":`,
		`"product_id":1`,
		`"quantity":2`,
		`"unit_price_paise":14900`,
		`"unit_price_rupees":149`,
		`"status":"PLACED"`,
		`"total_paise":29800`,
		`"total_rupees":298`,
		`"created_at":`,
	}
	for _, key := range expectedOrderKeys {
		if !strings.Contains(string(orderData), key) {
			t.Errorf("Order JSON missing key snippet %s, got: %s", key, string(orderData))
		}
	}

	// 3. Verify Request and Error shapes
	req := CreateOrderRequest{
		Items: []CreateOrderItemRequest{
			{ProductID: 1, Quantity: 2},
		},
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CreateOrderRequest: %v", err)
	}
	if !strings.Contains(string(reqData), `"product_id":1`) || !strings.Contains(string(reqData), `"quantity":2`) {
		t.Errorf("CreateOrderRequest JSON malformed: %s", string(reqData))
	}

	errResp := ErrorResponse{Error: "item unavailable"}
	errRespData, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("failed to marshal ErrorResponse: %v", err)
	}
	expectedErrJSON := `{"error":"item unavailable"}`
	if string(errRespData) != expectedErrJSON {
		t.Errorf("expected %s, got %s", expectedErrJSON, string(errRespData))
	}
}

// TestStore_BasicOperations tests seeding, listing, getting, creating, and updating products and orders.
func TestStore_BasicOperations(t *testing.T) {
	s := NewStore()

	// 1. Check seeded products
	prods := s.ListProducts()
	if len(prods) != 4 {
		t.Fatalf("expected 4 seeded products, got %d", len(prods))
	}
	// Brownie must be available=false
	p4, err := s.GetProduct(4)
	if err != nil {
		t.Fatalf("unexpected error getting product 4: %v", err)
	}
	if p4.Available {
		t.Errorf("expected product 4 to be unavailable")
	}

	// Non-existent product
	_, err = s.GetProduct(999)
	if err == nil || !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for non-existent product, got: %v", err)
	}

	// 2. Create order
	o1, err := s.CreateOrder(Order{
		Items:      []OrderItem{{ProductID: 1, Name: "Veg Burger", Quantity: 1, UnitPricePaise: 14900}},
		Status:     StatusPlaced,
		TotalPaise: 14900,
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	if o1.ID != 1 {
		t.Errorf("expected first order ID to be 1, got %d", o1.ID)
	}

	// 3. Get order
	fetched, err := s.GetOrder(1)
	if err != nil {
		t.Fatalf("failed to get order 1: %v", err)
	}
	if fetched.ID != 1 || fetched.Status != StatusPlaced {
		t.Errorf("unexpected order data: %+v", fetched)
	}

	// 4. Update order
	fetched.Status = StatusConfirmed
	updated, err := s.UpdateOrder(fetched)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}
	if updated.Status != StatusConfirmed {
		t.Errorf("expected updated status CONFIRMED, got %s", updated.Status)
	}

	// 5. Update non-existent order
	_, err = s.UpdateOrder(Order{ID: 999})
	if err == nil || !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound when updating non-existent order, got %v", err)
	}
}

// TestStore_DeepCopyEnsuresEncapsulation ensures modifying caller data does not corrupt the store.
func TestStore_DeepCopyEnsuresEncapsulation(t *testing.T) {
	s := NewStore()

	initialItems := []OrderItem{{ProductID: 1, Name: "Veg Burger", Quantity: 1, UnitPricePaise: 14900}}
	created, err := s.CreateOrder(Order{
		Items:      initialItems,
		Status:     StatusPlaced,
		TotalPaise: 14900,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Mutate the local slice passed into CreateOrder
	initialItems[0].Quantity = 999
	// Mutate the returned order's slice
	created.Items[0].Quantity = 888

	// Verify the stored order remains completely untouched
	stored, err := s.GetOrder(created.ID)
	if err != nil {
		t.Fatalf("failed to get stored order: %v", err)
	}
	if stored.Items[0].Quantity != 1 {
		t.Fatalf("data leak! Stored order was mutated to %d", stored.Items[0].Quantity)
	}
}

// TestStore_ConcurrentCreateOrder runs 50 goroutines simultaneously to test thread-safety
// and ensure that every order receives a strictly unique ID with zero race conditions.
func TestStore_ConcurrentCreateOrder(t *testing.T) {
	s := NewStore()
	const numOrders = 50

	// sync.WaitGroup allows us to wait for all 50 goroutines to complete their execution.
	var wg sync.WaitGroup
	wg.Add(numOrders)

	// sync.Map or mutex-protected map to collect assigned IDs across goroutines.
	idMap := sync.Map{}

	for i := 0; i < numOrders; i++ {
		// Launch a concurrent goroutine for each order creation
		go func() {
			defer wg.Done()

			order, err := s.CreateOrder(Order{
				Items:      []OrderItem{{ProductID: 1, Name: "Veg Burger", Quantity: 1, UnitPricePaise: 14900}},
				Status:     StatusPlaced,
				TotalPaise: 14900,
				CreatedAt:  time.Now().UTC(),
			})
			if err != nil {
				t.Errorf("unexpected error creating order in goroutine: %v", err)
				return
			}

			// Store the ID into our sync.Map. LoadOrStore returns loaded=true if the key was already present.
			if _, loaded := idMap.LoadOrStore(order.ID, true); loaded {
				t.Errorf("duplicate order ID generated: %d", order.ID)
			}
		}()
	}

	// Wait for all 50 goroutines to finish.
	wg.Wait()

	// Verify all 50 orders exist in the store
	orders := s.ListOrders()
	if len(orders) != numOrders {
		t.Fatalf("expected %d orders, found %d", numOrders, len(orders))
	}

	// Verify IDs are consecutive from 1 to 50
	for idx, o := range orders {
		expectedID := idx + 1
		if o.ID != expectedID {
			t.Errorf("order at index %d expected ID %d, got %d", idx, expectedID, o.ID)
		}
	}
}

// TestService_CreateOrder validates pricing, duplicate merging, and business validation rules.
func TestService_CreateOrder(t *testing.T) {
	store := NewStore()
	svc := NewService(store)

	// Case 1: Standard valid order (2 x Product 1 Veg Burger = 29800 paise / 298 rupees)
	{
		order, err := svc.CreateOrder(CreateOrderRequest{
			Items: []CreateOrderItemRequest{
				{ProductID: 1, Quantity: 2},
			},
		})
		if err != nil {
			t.Fatalf("case 1: unexpected error: %v", err)
		}
		if order.Status != StatusPlaced {
			t.Errorf("case 1: expected status PLACED, got %s", order.Status)
		}
		if order.TotalPaise != 29800 {
			t.Errorf("case 1: expected total_paise 29800, got %d", order.TotalPaise)
		}
		if order.TotalRupees != 298.0 {
			t.Errorf("case 1: expected total_rupees 298, got %f", order.TotalRupees)
		}
		if len(order.Items) != 1 || order.Items[0].Quantity != 2 || order.Items[0].Name != "Veg Burger" {
			t.Errorf("case 1: unexpected order items: %+v", order.Items)
		}
	}

	// Case 2: Duplicate product_id merging (2 x Product 1 + 3 x Product 1 = 5 x 14900 = 74500 paise)
	{
		order, err := svc.CreateOrder(CreateOrderRequest{
			Items: []CreateOrderItemRequest{
				{ProductID: 1, Quantity: 2},
				{ProductID: 1, Quantity: 3},
			},
		})
		if err != nil {
			t.Fatalf("case 2: unexpected error: %v", err)
		}
		if len(order.Items) != 1 {
			t.Fatalf("case 2: expected 1 merged item, got %d", len(order.Items))
		}
		if order.Items[0].Quantity != 5 {
			t.Errorf("case 2: expected merged quantity 5, got %d", order.Items[0].Quantity)
		}
		if order.TotalPaise != 74500 {
			t.Errorf("case 2: expected total_paise 74500, got %d", order.TotalPaise)
		}
	}

	// Case 3: Empty items list should be rejected with validation error
	{
		_, err := svc.CreateOrder(CreateOrderRequest{Items: []CreateOrderItemRequest{}})
		if err == nil {
			t.Fatalf("case 3: expected error for empty items list, got nil")
		}
		var valErr *ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("case 3: expected ValidationError, got %T: %v", err, err)
		}
	}

	// Case 4: Quantity < 1 should be rejected
	{
		_, err := svc.CreateOrder(CreateOrderRequest{
			Items: []CreateOrderItemRequest{
				{ProductID: 1, Quantity: 0},
			},
		})
		if err == nil {
			t.Fatalf("case 4: expected error for quantity 0, got nil")
		}
		var valErr *ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("case 4: expected ValidationError, got %T: %v", err, err)
		}
	}

	// Case 5: Non-existent product ID returns ErrNotFound
	{
		_, err := svc.CreateOrder(CreateOrderRequest{
			Items: []CreateOrderItemRequest{
				{ProductID: 999, Quantity: 1},
			},
		})
		if err == nil || !errors.Is(err, ErrNotFound) {
			t.Errorf("case 5: expected ErrNotFound for product 999, got %v", err)
		}
	}

	// Case 6: Unavailable product (Product 4 Chocolate Brownie) returns ValidationError
	{
		_, err := svc.CreateOrder(CreateOrderRequest{
			Items: []CreateOrderItemRequest{
				{ProductID: 4, Quantity: 1},
			},
		})
		if err == nil {
			t.Fatalf("case 6: expected error for unavailable product, got nil")
		}
		var valErr *ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("case 6: expected ValidationError for unavailable product, got %v", err)
		}
	}
}

// TestCanTransition_TableDriven performs table-driven testing of the pure CanTransition function,
// checking every valid forward transition and numerous invalid/backwards transitions.
func TestCanTransition_TableDriven(t *testing.T) {
	// Table-driven tests are the standard idiom in Go: declare a slice of anonymous test structs
	// and iterate over them using t.Run for clean, descriptive test reporting.
	tests := []struct {
		name     string
		from     OrderStatus
		to       OrderStatus
		expected bool
	}{
		// Valid transitions
		{name: "PLACED to CONFIRMED", from: StatusPlaced, to: StatusConfirmed, expected: true},
		{name: "CONFIRMED to PREPARING", from: StatusConfirmed, to: StatusPreparing, expected: true},
		{name: "PREPARING to OUT_FOR_DELIVERY", from: StatusPreparing, to: StatusOutForDelivery, expected: true},
		{name: "OUT_FOR_DELIVERY to DELIVERED", from: StatusOutForDelivery, to: StatusDelivered, expected: true},

		// Backward transitions (must be disallowed)
		{name: "CONFIRMED to PLACED", from: StatusConfirmed, to: StatusPlaced, expected: false},
		{name: "PREPARING to CONFIRMED", from: StatusPreparing, to: StatusConfirmed, expected: false},
		{name: "OUT_FOR_DELIVERY to PREPARING", from: StatusOutForDelivery, to: StatusPreparing, expected: false},
		{name: "DELIVERED to PREPARING", from: StatusDelivered, to: StatusPreparing, expected: false},
		{name: "DELIVERED to PLACED", from: StatusDelivered, to: StatusPlaced, expected: false},

		// Terminal states cannot transition to anything
		{name: "DELIVERED to OUT_FOR_DELIVERY", from: StatusDelivered, to: StatusOutForDelivery, expected: false},
		{name: "CANCELLED to PLACED", from: StatusCancelled, to: StatusPlaced, expected: false},
		{name: "CANCELLED to CONFIRMED", from: StatusCancelled, to: StatusConfirmed, expected: false},

		// Skipped stages (e.g. jumping straight from PLACED to DELIVERED)
		{name: "PLACED to DELIVERED", from: StatusPlaced, to: StatusDelivered, expected: false},
		{name: "PLACED to OUT_FOR_DELIVERY", from: StatusPlaced, to: StatusOutForDelivery, expected: false},

		// Self transitions
		{name: "PLACED to PLACED", from: StatusPlaced, to: StatusPlaced, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := CanTransition(tc.from, tc.to)
			if actual != tc.expected {
				t.Errorf("CanTransition(%s, %s) = %v; expected %v", tc.from, tc.to, actual, tc.expected)
			}
		})
	}
}

// TestCanCancel_TableDriven tests the cancellation rule: only PLACED and CONFIRMED can be cancelled.
func TestCanCancel_TableDriven(t *testing.T) {
	tests := []struct {
		status   OrderStatus
		expected bool
	}{
		{status: StatusPlaced, expected: true},
		{status: StatusConfirmed, expected: true},
		{status: StatusPreparing, expected: false},
		{status: StatusOutForDelivery, expected: false},
		{status: StatusDelivered, expected: false},
		{status: StatusCancelled, expected: false},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			actual := CanCancel(tc.status)
			if actual != tc.expected {
				t.Errorf("CanCancel(%s) = %v; expected %v", tc.status, actual, tc.expected)
			}
		})
	}
}

// TestService_StatusTransitionsAndCancellation tests service methods UpdateOrderStatus and CancelOrder.
func TestService_StatusTransitionsAndCancellation(t *testing.T) {
	store := NewStore()
	svc := NewService(store)

	order, err := svc.CreateOrder(CreateOrderRequest{
		Items: []CreateOrderItemRequest{{ProductID: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("failed to create initial order: %v", err)
	}

	// 1. Advance PLACED -> CONFIRMED
	order, err = svc.UpdateOrderStatus(order.ID, StatusConfirmed)
	if err != nil {
		t.Fatalf("failed transition to CONFIRMED: %v", err)
	}
	if order.Status != StatusConfirmed {
		t.Errorf("expected CONFIRMED, got %s", order.Status)
	}

	// 2. Reject passing CANCELLED to UpdateOrderStatus
	_, err = svc.UpdateOrderStatus(order.ID, StatusCancelled)
	if err == nil {
		t.Fatalf("expected error when passing CANCELLED to UpdateOrderStatus")
	}

	// 3. Reject invalid transition CONFIRMED -> DELIVERED (skipping stages)
	_, err = svc.UpdateOrderStatus(order.ID, StatusDelivered)
	if err == nil || !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition for CONFIRMED -> DELIVERED, got %v", err)
	}

	// 4. Cancel from CONFIRMED is allowed
	order, err = svc.CancelOrder(order.ID)
	if err != nil {
		t.Fatalf("failed to cancel CONFIRMED order: %v", err)
	}
	if order.Status != StatusCancelled {
		t.Errorf("expected CANCELLED, got %s", order.Status)
	}

	// 5. Cancelling an already CANCELLED order is disallowed (returns ErrInvalidTransition)
	_, err = svc.CancelOrder(order.ID)
	if err == nil || !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition when cancelling already cancelled order, got %v", err)
	}
}



