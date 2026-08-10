package repository

import (
	"database/sql"
	"encoding/json"
	"time"
)

type PremiumRepository struct {
	db *sql.DB
}

func NewPremiumRepository(db *sql.DB) *PremiumRepository {
	return &PremiumRepository{db: db}
}

func (r *PremiumRepository) GetPlans() ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`SELECT id, name, price, duration, features FROM premium_plans ORDER BY price`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []map[string]interface{}
	for rows.Next() {
		var id, name string
		var price float64
		var duration int
		var featuresJSON []byte

		if err := rows.Scan(&id, &name, &price, &duration, &featuresJSON); err != nil {
			return nil, err
		}
		features := []string{}
		json.Unmarshal(featuresJSON, &features)

		plans = append(plans, map[string]interface{}{
			"id": id, "name": name, "price": price,
			"duration": duration, "features": features,
		})
	}
	return plans, nil
}

func (r *PremiumRepository) GetSubscription(userID string) (map[string]interface{}, error) {
	var planID, planName string
	var startsAt, endsAt time.Time
	var autoRenew bool

	err := r.db.QueryRow(`SELECT p.id, p.name, ps.starts_at, ps.ends_at, ps.auto_renew
		FROM premium_subscriptions ps JOIN premium_plans p ON ps.plan_id = p.id
		WHERE ps.user_id = $1 AND ps.ends_at > NOW()`, userID).
		Scan(&planID, &planName, &startsAt, &endsAt, &autoRenew)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return map[string]interface{}{
		"plan_id": planID, "plan_name": planName,
		"starts_at": startsAt, "ends_at": endsAt,
		"auto_renew": autoRenew, "active": endsAt.After(time.Now()),
	}, nil
}

func (r *PremiumRepository) Subscribe(userID, planID string) error {
	var duration int
	err := r.db.QueryRow(`SELECT duration FROM premium_plans WHERE id = $1`, planID).Scan(&duration)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`INSERT INTO premium_subscriptions (user_id, plan_id, starts_at, ends_at, auto_renew)
		VALUES ($1, $2, NOW(), NOW() + make_interval(days := $3), true)
		ON CONFLICT (user_id) DO UPDATE SET plan_id = $2, starts_at = NOW(), ends_at = NOW() + make_interval(days := $3), auto_renew = true`,
		userID, planID, duration)
	return err
}

func (r *PremiumRepository) Cancel(userID string) error {
	_, err := r.db.Exec(`UPDATE premium_subscriptions SET auto_renew = false WHERE user_id = $1`, userID)
	return err
}

func (r *PremiumRepository) Check(userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM premium_subscriptions WHERE user_id = $1 AND ends_at > NOW())`, userID).Scan(&exists)
	return exists, err
}

func (r *PremiumRepository) GetPlan(planID string) (map[string]interface{}, error) {
	var id, name string
	var price float64
	var duration int
	err := r.db.QueryRow(`SELECT id, name, price, duration FROM premium_plans WHERE id = $1`, planID).
		Scan(&id, &name, &price, &duration)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "name": name, "price": price, "duration": duration,
	}, nil
}

func (r *PremiumRepository) CreatePayment(userID, planID string, amount float64, status, stripeSessionID string) error {
	_, err := r.db.Exec(`INSERT INTO premium_payments (user_id, plan_id, amount, status, stripe_session_id)
		VALUES ($1, $2, $3, $4, $5)`,
		userID, planID, amount, status, stripeSessionID)
	return err
}

func (r *PremiumRepository) MarkPaymentCompleted(stripeSessionID string) error {
	_, err := r.db.Exec(`UPDATE premium_payments SET status = 'completed' WHERE stripe_session_id = $1`, stripeSessionID)
	return err
}

func (r *PremiumRepository) FindPaymentBySession(stripeSessionID string) (userID, planID string, err error) {
	err = r.db.QueryRow(`SELECT user_id, plan_id FROM premium_payments WHERE stripe_session_id = $1`, stripeSessionID).
		Scan(&userID, &planID)
	return
}
