package handler

import (
	"encoding/json"
	"net/http"

	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
)

type StripeConfig struct {
	SecretKey     string
	WebhookSecret string
	SuccessURL    string
	CancelURL     string
}

type PremiumHandler struct {
	premiumRepo *repository.PremiumRepository
	stripe      *StripeConfig
}

func NewPremiumHandler(pr *repository.PremiumRepository, sc *StripeConfig) *PremiumHandler {
	if sc == nil {
		sc = &StripeConfig{}
	}
	return &PremiumHandler{premiumRepo: pr, stripe: sc}
}

func (h *PremiumHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.premiumRepo.GetPlans()
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, plans)
}

func (h *PremiumHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	sub, err := h.premiumRepo.GetSubscription(claims.UserID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sub == nil {
		utils.SuccessResponse(w, map[string]interface{}{"active": false})
		return
	}
	utils.SuccessResponse(w, sub)
}

func (h *PremiumHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlanID == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "plan_id required")
		return
	}

	plan, err := h.premiumRepo.GetPlan(req.PlanID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "plan not found")
		return
	}

	// Mock-режим: без Stripe-ключей просто активируем подписку (dev).
	if h.stripe.SecretKey == "" {
		if err := h.premiumRepo.Subscribe(claims.UserID, req.PlanID); err != nil {
			utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		utils.SuccessResponse(w, map[string]interface{}{
			"status":    "subscribed",
			"mode":      "mock",
			"plan_id":   plan["id"],
			"plan_name": plan["name"],
		})
		return
	}

	stripe.Key = h.stripe.SecretKey

	price, ok := plan["price"].(float64)
	if !ok {
		price = 0
	}
	amount := int64(price * 100)

	ps, err := session.New(&stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(h.stripe.SuccessURL),
		CancelURL:  stripe.String(h.stripe.CancelURL),
		Metadata: map[string]string{
			"user_id": claims.UserID,
			"plan_id": req.PlanID,
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("rub"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(plan["name"].(string)),
					},
					UnitAmount: stripe.Int64(amount),
				},
				Quantity: stripe.Int64(1),
			},
		},
	})
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Фиксируем платеж в статусе pending.
	if err := h.premiumRepo.CreatePayment(claims.UserID, req.PlanID, price, "pending", ps.ID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"status":       "checkout_created",
		"checkout_url": ps.URL,
		"session_id":   ps.ID,
	})
}

func (h *PremiumHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if err := h.premiumRepo.Cancel(claims.UserID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "cancelled"})
}

func (h *PremiumHandler) Webhook(w http.ResponseWriter, r *http.Request) {
	if h.stripe.WebhookSecret == "" {
		utils.ErrorResponse(w, http.StatusInternalServerError, "webhook not configured")
		return
	}

	payload := make([]byte, 0, 1<<20)
	if r.Body != nil {
		buf := make([]byte, 0, 1<<20)
		payload = make([]byte, 0)
		chunk := make([]byte, 1<<15)
		for {
			n, err := r.Body.Read(chunk)
			payload = append(payload, chunk[:n]...)
			if err != nil {
				break
			}
		}
		_ = buf
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.stripe.WebhookSecret)
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if event.Type != "checkout.session.completed" {
		utils.SuccessResponse(w, map[string]string{"received": "true"})
		return
	}

	var cs stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	userID, ok := cs.Metadata["user_id"]
	planID, ok2 := cs.Metadata["plan_id"]
	if !ok || !ok2 {
		utils.ErrorResponse(w, http.StatusBadRequest, "missing metadata")
		return
	}

	if err := h.premiumRepo.Subscribe(userID, planID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.premiumRepo.MarkPaymentCompleted(cs.ID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"status": "activated"})
}
