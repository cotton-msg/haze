package repository

import "database/sql"

type StatsRepository struct {
	db *sql.DB
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

type DashboardStats struct {
	UsersTotal    int `json:"users_total"`
	ChatsTotal    int `json:"chats_total"`
	MessagesToday int `json:"messages_today"`
	PremiumActive int `json:"premium_active"`
	BotsTotal     int `json:"bots_total"`
	ActiveCalls   int `json:"active_calls"`
}

func (r *StatsRepository) GetDashboard() (*DashboardStats, error) {
	stats := &DashboardStats{}

	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.UsersTotal); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM chats`).Scan(&stats.ChatsTotal); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE created_at >= NOW() - INTERVAL '1 day'`).Scan(&stats.MessagesToday); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM premium_subscriptions WHERE ends_at > NOW()`).Scan(&stats.PremiumActive); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM bots`).Scan(&stats.BotsTotal); err != nil {
		return nil, err
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM calls WHERE status IN ('ringing', 'active')`).Scan(&stats.ActiveCalls); err != nil {
		return nil, err
	}

	return stats, nil
}
