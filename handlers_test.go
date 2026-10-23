package main

// Package main: provides HTTP endpoint tests using Go's standard library net/http/httptest package.

import (
	// encoding/json unmarshals JSON response bodies in test assertions.
	"encoding/json"
	// net/http provides HTTP status codes and methods.
	"net/http"
	// net/http/httptest provides synthetic HTTP request and response recorder implementations
	// allowing us to test HTTP handlers in-memory without binding to an actual network port.
	"net/http/httptest"
	// strings provides string manipulation and string reader for request payloads.
	"strings"
	// testing provides Go's automated test framework primitives.
	"testing"
)

// setupTestServer helper initializes an isolated test stack (Store -> Service -> Handler -> ServeMux)
// for each test run so tests do not share state or interfere with each other.
func setupTestServer() *http.ServeMux {
	store := NewStore()
	service := NewService(store)
	handler := NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

// TestHandleHealth verifies that GET /health returns 200 OK with {"status":"ok"}.
func TestHandleHealth(t *testing.T) {
	mux := setupTestServer()

	// httptest.NewRequest creates a mock *http.Request.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	// httptest.NewRecorder records the HTTP response status, headers, and body produced by the handler.
	rec := httptest.NewRecorder()

	// Dispatch request directly to the ServeMux router.
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", body["status"])
	}
}

// TestHandleListProducts verifies GET /products returns 200 OK and all 4 seeded products.
func TestHandleListProducts(t *testing.T) {
	mux := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/products", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var products []Product
	if err := json.Unmarshal(rec.Body.Bytes(), &products); err != nil {
		t.Fatalf("failed to decode products: %v", err)
	}

	if len(products) != 4 {
		t.Fatalf("expected 4 products, got %d", len(products))
	}
	if products[0].Name != "Veg Burger" || products[0].PricePaise != 14900 {
		t.Errorf("unexpected first product: %+v", products[0])
	}
}

// TestHandleGetProduct verifies GET /products/{id} for success, 404, and 400 cases.
func TestHandleGetProduct(t *testing.T) {
	mux := setupTestServer()

	// Case 1: Valid product ID returns 200 OK
	{
		req := httptest.NewRequest(http.MethodGet, "/products/1", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("case 1: expected status 200, got %d", rec.Code)
		}

		var p Product
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("case 1: failed to parse product: %v", err)
		}
		if p.ID != 1 || p.Name != "Veg Burger" {
			t.Errorf("case 1: unexpected product returned: %+v", p)
		}
	}

	// Case 2: Non-existent product ID returns 404 Not Found
	{
		req := httptest.NewRequest(http.MethodGet, "/products/999", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("case 2: expected status 404, got %d", rec.Code)
		}

		var errResp ErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("case 2: failed to parse error response: %v", err)
		}
		if errResp.Error != "resource not found" {
			t.Errorf("case 2: expected error 'resource not found', got '%s'", errResp.Error)
		}
	}

	// Case 3: Non-numeric product ID returns 400 Bad Request
	{
		req := httptest.NewRequest(http.MethodGet, "/products/burger", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 3: expected status 400, got %d", rec.Code)
		}

		var errResp ErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("case 3: failed to parse error response: %v", err)
		}
		if errResp.Error != "product id must be an integer" {
			t.Errorf("case 3: expected error 'product id must be an integer', got '%s'", errResp.Error)
		}
	}
}

// TestHandleCreateOrder validates the POST /orders endpoint for success, validation, and error cases.
func TestHandleCreateOrder(t *testing.T) {
	mux := setupTestServer()

	// 1. Success case: Ordering 2 x Product 1 (Veg Burger) returns 201 Created with 29800 total paise
	{
		body := `{"items":[{"product_id":1,"quantity":2}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("case 1: expected status 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var created Order
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("case 1: failed to decode order: %v", err)
		}
		if created.ID == 0 {
			t.Errorf("case 1: expected valid non-zero order ID")
		}
		if created.Status != StatusPlaced {
			t.Errorf("case 1: expected status PLACED, got %s", created.Status)
		}
		if created.TotalPaise != 29800 {
			t.Errorf("case 1: expected total_paise 29800, got %d", created.TotalPaise)
		}
		if created.TotalRupees != 298.0 {
			t.Errorf("case 1: expected total_rupees 298.0, got %f", created.TotalRupees)
		}
	}

	// 2. Reject malformed JSON -> 400
	{
		body := `{"items": [{"product_id":1, quantity:}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 2: expected status 400 for malformed JSON, got %d", rec.Code)
		}
	}

	// 3. Reject unknown fields -> 400
	{
		body := `{"items":[{"product_id":1,"quantity":2}],"hacked_price":10}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 3: expected status 400 for unknown field, got %d", rec.Code)
		}
	}

	// 4. Reject empty items -> 400
	{
		body := `{"items":[]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 4: expected status 400 for empty items, got %d", rec.Code)
		}
	}

	// 5. Reject invalid quantity (0) -> 400
	{
		body := `{"items":[{"product_id":1,"quantity":0}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 5: expected status 400 for quantity 0, got %d", rec.Code)
		}
	}

	// 6. Non-existent product -> 404
	{
		body := `{"items":[{"product_id":999,"quantity":1}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("case 6: expected status 404 for unknown product, got %d", rec.Code)
		}
	}

	// 7. Unavailable product (Product 4 Chocolate Brownie) -> 400
	{
		body := `{"items":[{"product_id":4,"quantity":1}]}`
		req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 7: expected status 400 for unavailable product, got %d", rec.Code)
		}
	}
}

// TestHandleListOrders validates GET /orders for empty state and populated state.
func TestHandleListOrders(t *testing.T) {
	mux := setupTestServer()

	// 1. When no orders exist, must return 200 OK with an empty array [] (never null).
	{
		req := httptest.NewRequest(http.MethodGet, "/orders", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		rawBody := strings.TrimSpace(rec.Body.String())
		if rawBody != "[]" {
			t.Fatalf("expected empty JSON array '[]', got '%s'", rawBody)
		}
	}

	// 2. Create an order, then verify it appears in GET /orders.
	{
		createBody := `{"items":[{"product_id":1,"quantity":1}]}`
		createReq := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(createBody))
		createRec := httptest.NewRecorder()
		mux.ServeHTTP(createRec, createReq)
		if createRec.Code != http.StatusCreated {
			t.Fatalf("failed to create order: %d", createRec.Code)
		}

		req := httptest.NewRequest(http.MethodGet, "/orders", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var orders []Order
		if err := json.Unmarshal(rec.Body.Bytes(), &orders); err != nil {
			t.Fatalf("failed to parse orders list: %v", err)
		}
		if len(orders) != 1 {
			t.Fatalf("expected 1 order in list, got %d", len(orders))
		}
		if orders[0].ID != 1 {
			t.Errorf("expected order ID 1, got %d", orders[0].ID)
		}
	}
}

// TestHandleGetOrder validates GET /orders/{id} for success, 404, and 400 cases.
func TestHandleGetOrder(t *testing.T) {
	mux := setupTestServer()

	// Seed one order
	createBody := `{"items":[{"product_id":1,"quantity":2}]}`
	createReq := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(createBody))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("failed to seed test order: %d", createRec.Code)
	}

	// Case 1: Valid order ID returns 200 OK with order JSON
	{
		req := httptest.NewRequest(http.MethodGet, "/orders/1", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("case 1: expected status 200, got %d", rec.Code)
		}

		var o Order
		if err := json.Unmarshal(rec.Body.Bytes(), &o); err != nil {
			t.Fatalf("case 1: failed to parse order: %v", err)
		}
		if o.ID != 1 || o.Status != StatusPlaced || o.TotalPaise != 29800 {
			t.Errorf("case 1: unexpected order data: %+v", o)
		}
	}

	// Case 2: Non-existent order ID returns 404 Not Found
	{
		req := httptest.NewRequest(http.MethodGet, "/orders/999", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("case 2: expected status 404, got %d", rec.Code)
		}
	}

	// Case 3: Non-numeric order ID returns 400 Bad Request
	{
		req := httptest.NewRequest(http.MethodGet, "/orders/abc", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("case 3: expected status 400, got %d", rec.Code)
		}

		var errResp ErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("case 3: failed to parse error response: %v", err)
		}
		if errResp.Error != "order id must be an integer" {
			t.Errorf("case 3: expected error 'order id must be an integer', got '%s'", errResp.Error)
		}
	}
}

// TestHandleUpdateOrderStatus tests the PATCH /orders/{id}/status endpoint.
func TestHandleUpdateOrderStatus(t *testing.T) {
	mux := setupTestServer()

	// 1. Seed an order (starts in PLACED)
	createReq := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"items":[{"product_id":1,"quantity":1}]}`))
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("failed to seed order: %d", createRec.Code)
	}

	// 2. Successful transition: PLACED -> CONFIRMED returns 200 OK
	{
		req := httptest.NewRequest(http.MethodPatch, "/orders/1/status", strings.NewReader(`{"status":"CONFIRMED"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var o Order
		_ = json.Unmarshal(rec.Body.Bytes(), &o)
		if o.Status != StatusConfirmed {
			t.Errorf("expected CONFIRMED, got %s", o.Status)
		}
	}

	// 3. Complete lifecycle to DELIVERED
	for _, nextStatus := range []string{"PREPARING", "OUT_FOR_DELIVERY", "DELIVERED"} {
		req := httptest.NewRequest(http.MethodPatch, "/orders/1/status", strings.NewReader(`{"status":"`+nextStatus+`"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("failed transition to %s, got %d: %s", nextStatus, rec.Code, rec.Body.String())
		}
	}

	// 4. Invalid backwards transition: DELIVERED -> PREPARING returns 409 Conflict
	{
		req := httptest.NewRequest(http.MethodPatch, "/orders/1/status", strings.NewReader(`{"status":"PREPARING"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict for DELIVERED -> PREPARING, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 5. Unknown status value returns 400 Bad Request
	{
		req := httptest.NewRequest(http.MethodPatch, "/orders/1/status", strings.NewReader(`{"status":"UNKNOWN_STATUS"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request for unknown status, got %d", rec.Code)
		}
	}

	// 6. Passing CANCELLED to PATCH status returns 400 Bad Request (cancel has its own endpoint)
	{
		req := httptest.NewRequest(http.MethodPatch, "/orders/1/status", strings.NewReader(`{"status":"CANCELLED"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request when setting CANCELLED via PATCH status, got %d", rec.Code)
		}
	}

	// 7. Non-existent order returns 404 Not Found
	{
		req := httptest.NewRequest(http.MethodPatch, "/orders/999/status", strings.NewReader(`{"status":"CONFIRMED"}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found for non-existent order, got %d", rec.Code)
		}
	}
}

// TestHandleCancelOrder tests the POST /orders/{id}/cancel endpoint.
func TestHandleCancelOrder(t *testing.T) {
	mux := setupTestServer()

	// Case 1: Cancel PLACED order succeeds -> 200 OK
	{
		createReq := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"items":[{"product_id":1,"quantity":1}]}`))
		createRec := httptest.NewRecorder()
		mux.ServeHTTP(createRec, createReq)
		var o Order
		_ = json.Unmarshal(createRec.Body.Bytes(), &o)

		req := httptest.NewRequest(http.MethodPost, "/orders/1/cancel", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK cancelling PLACED order, got %d: %s", rec.Code, rec.Body.String())
		}
		var cancelled Order
		_ = json.Unmarshal(rec.Body.Bytes(), &cancelled)
		if cancelled.Status != StatusCancelled {
			t.Errorf("expected status CANCELLED, got %s", cancelled.Status)
		}
	}

	// Case 2: Cancelling an order already in PREPARING returns 409 Conflict
	{
		// Seed second order
		createReq := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"items":[{"product_id":1,"quantity":1}]}`))
		createRec := httptest.NewRecorder()
		mux.ServeHTTP(createRec, createReq)

		// Advance to CONFIRMED -> PREPARING
		patchReq1 := httptest.NewRequest(http.MethodPatch, "/orders/2/status", strings.NewReader(`{"status":"CONFIRMED"}`))
		patchRec1 := httptest.NewRecorder()
		mux.ServeHTTP(patchRec1, patchReq1)

		patchReq2 := httptest.NewRequest(http.MethodPatch, "/orders/2/status", strings.NewReader(`{"status":"PREPARING"}`))
		patchRec2 := httptest.NewRecorder()
		mux.ServeHTTP(patchRec2, patchReq2)

		// Now attempt to cancel PREPARING order
		cancelReq := httptest.NewRequest(http.MethodPost, "/orders/2/cancel", nil)
		cancelRec := httptest.NewRecorder()
		mux.ServeHTTP(cancelRec, cancelReq)

		if cancelRec.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict cancelling PREPARING order, got %d: %s", cancelRec.Code, cancelRec.Body.String())
		}
	}

	// Case 3: Cancelling non-existent order returns 404 Not Found
	{
		cancelReq := httptest.NewRequest(http.MethodPost, "/orders/999/cancel", nil)
		cancelRec := httptest.NewRecorder()
		mux.ServeHTTP(cancelRec, cancelReq)

		if cancelRec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found for missing order, got %d", cancelRec.Code)
		}
	}
}



