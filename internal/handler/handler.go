package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"recommendation-service/internal/model"
	"recommendation-service/internal/service"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes wires up all HTTP routes and middleware.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/users/{user_id}/recommendations", h.GetRecommendations)
	r.Get("/recommendations/batch", h.GetBatchRecommendations)

	return r
}

// GetRecommendations handles GET /users/{user_id}/recommendations
func (h *Handler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	// Parse path param
	userID, err := strconv.ParseInt(chi.URLParam(r, "user_id"), 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid user_id parameter")
		return
	}

	// Parse query param: limit (default 10, max 50)
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit <= 0 || limit > 50 {
			writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid limit parameter")
			return
		}
	}

	resp, err := h.svc.GetRecommendations(r.Context(), userID, limit)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "user_not_found",
				"User with ID "+strconv.FormatInt(userID, 10)+" does not exist")
		case errors.Is(err, service.ErrModelUnavailable):
			writeError(w, http.StatusServiceUnavailable, "model_unavailable",
				"Recommendation model is temporarily unavailable")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error",
				"An unexpected error occurred")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetBatchRecommendations handles GET /recommendations/batch
func (h *Handler) GetBatchRecommendations(w http.ResponseWriter, r *http.Request) {
	page := 1
	limit := 20
	var err error

	if v := r.URL.Query().Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil || page < 1 {
			writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid page parameter")
			return
		}
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "invalid_parameter", "Invalid limit parameter")
			return
		}
	}

	resp, err := h.svc.GetBatchRecommendations(r.Context(), page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error",
			"An unexpected error occurred")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, model.ErrorResponse{Error: code, Message: msg})
}
