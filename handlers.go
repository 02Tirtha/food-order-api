package main

// Package main: provides HTTP request handlers, routing registration, middleware, and JSON helpers.

import (
	// encoding/json provides JSON encoding and decoding capabilities.
	"encoding/json"
	// errors package is used to inspect error types with errors.Is.
	"errors"
	// log package logs operational and error details to the server console.
	"log"
	// net/http provides HTTP status codes, request/response types, and ServeMux.
	"net/http"
	// strconv provides conversions to and from string representations of basic data types.
	"strconv"
	// time package allows measuring request processing durations in our logging middleware.
	"time"
)

// Handler holds dependencies required by HTTP endpoints, specifically the business Service.
type Handler struct {
	service *Service
}

// NewHandler constructs a Handler instance with the given Service dependency.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes wires each HTTP method and path pattern to its corresponding handler function.
// Using Go 1.22+ pattern routing, methods and path parameters are specified directly in the pattern string.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /products", h.handleListProducts)
	mux.HandleFunc("GET /products/{id}", h.handleGetProduct)
	mux.HandleFunc("POST /orders", h.handleCreateOrder)
	mux.HandleFunc("GET /orders", h.handleListOrders)
	mux.HandleFunc("GET /orders/{id}", h.handleGetOrder)
	mux.HandleFunc("PATCH /orders/{id}/status", h.handleUpdateOrderStatus)
	mux.HandleFunc("POST /orders/{id}/cancel", h.handleCancelOrder)
}

// handleHealth responds with {"status":"ok"} to confirm the server is responsive.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleUpdateOrderStatus handles PATCH /orders/{id}/status to transition order status forward.
func (h *Handler) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "order id must be an integer")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req UpdateStatusRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	order, err := h.service.UpdateOrderStatus(id, req.Status)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// handleCancelOrder handles POST /orders/{id}/cancel to cancel an active order.
func (h *Handler) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "order id must be an integer")
		return
	}

	order, err := h.service.CancelOrder(id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// handleListOrders handles GET /orders and returns all orders sorted by ID ascending.
func (h *Handler) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders := h.service.ListOrders()
	respondJSON(w, http.StatusOK, orders)
}

// handleGetOrder handles GET /orders/{id}.
// Returns 200 with the order, 400 for a non-numeric ID, or 404 if the order does not exist.
func (h *Handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "order id must be an integer")
		return
	}

	order, err := h.service.GetOrder(id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// handleListProducts handles GET /products and returns the full list of products.
func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	products := h.service.ListProducts()
	respondJSON(w, http.StatusOK, products)
}

// handleGetProduct handles GET /products/{id}.
// Returns 200 with the product, 400 if the ID is not an integer, or 404 if the product is not found.
func (h *Handler) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	// r.PathValue is a Go 1.22 feature that extracts wildcard variables from the URL pattern.
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		// If the client supplies a non-numeric ID (e.g. /products/abc), return HTTP 400 Bad Request.
		respondError(w, http.StatusBadRequest, "product id must be an integer")
		return
	}

	product, err := h.service.GetProduct(id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, product)
}

// handleCreateOrder handles POST /orders to place a new food order.
func (h *Handler) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	// Enforce 1 MB request body limit to protect against memory exhaustion attacks.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req CreateOrderRequest
	dec := json.NewDecoder(r.Body)
	// DisallowUnknownFields ensures extra unrecognized keys trigger a 400 rejection.
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	order, err := h.service.CreateOrder(req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	// 201 Created indicates successful creation of the order resource.
	respondJSON(w, http.StatusCreated, order)
}

// writeServiceError maps service-level errors to their respective HTTP status codes in ONE centralized location.
func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		respondError(w, http.StatusBadRequest, valErr.Message)
		return
	}
	if errors.Is(err, ErrNotFound) {
		respondError(w, http.StatusNotFound, "resource not found")
		return
	}
	if errors.Is(err, ErrInvalidTransition) {
		respondError(w, http.StatusConflict, err.Error())
		return
	}
	// Log the actual internal error server-side, but keep client responses generic to avoid leaking internals.
	log.Printf("ERROR: unhandled internal error: %v", err)
	respondError(w, http.StatusInternalServerError, "internal server error")
}

// respondJSON writes the HTTP status code, sets the Content-Type header to application/json,
// and streams the serialized data into the response writer.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log JSON serialization issues server-side without exposing internal details to clients.
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}

// respondError formats and writes an error response adhering to the exact required shape:
// {"error": "human readable message"}
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

// statusResponseWriter is an HTTP response wrapper that intercepts and records the status code
// written by handlers, enabling our logging middleware to report accurate HTTP status codes.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before forwarding it to the underlying ResponseWriter.
func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs the HTTP method, request URL path, response status code,
// and execution duration for every incoming request.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Default to 200 OK in case the handler writes directly to the body without an explicit WriteHeader call.
		wrappedWriter := &statusResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, wrappedWriter.statusCode, duration)
	})
}
