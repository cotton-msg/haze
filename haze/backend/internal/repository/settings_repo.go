package repository

import (
	"database/sql"
	"encoding/json"
)

type UserSettingsRepository struct {
	db *sql.DB
}

func NewUserSettingsRepository(db *sql.DB) *UserSettingsRepository {
	return &UserSettingsRepository{db: db}
}

func (r *UserSettingsRepository) Get(userID string) (map[string]interface{}, error) {
	var themeID *string
	var wallpaperURL *string
	var notifSoundsJSON []byte

	query := `SELECT theme_id, wallpaper_url, notification_sounds FROM user_settings WHERE user_id = $1`
	err := r.db.QueryRow(query, userID).Scan(&themeID, &wallpaperURL, &notifSoundsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}

	settings := map[string]interface{}{}
	if themeID != nil {
		settings["theme_id"] = *themeID
	}
	if wallpaperURL != nil {
		settings["wallpaper_url"] = *wallpaperURL
	}
	if notifSoundsJSON != nil {
		var sounds map[string]interface{}
		json.Unmarshal(notifSoundsJSON, &sounds)
		settings["notification_sounds"] = sounds
	}
	return settings, nil
}

func (r *UserSettingsRepository) Upsert(userID, themeID, wallpaperURL string, notifSounds map[string]interface{}) error {
	soundsJSON, _ := json.Marshal(notifSounds)
	query := `INSERT INTO user_settings (user_id, theme_id, wallpaper_url, notification_sounds)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET theme_id = $2, wallpaper_url = $3, notification_sounds = $4`
	_, err := r.db.Exec(query, userID, nullable(themeID), nullable(wallpaperURL), soundsJSON)
	return err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type ThemeRepository struct {
	db *sql.DB
}

func NewThemeRepository(db *sql.DB) *ThemeRepository {
	return &ThemeRepository{db: db}
}

func (r *ThemeRepository) List() ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`SELECT id, name, is_premium, colors FROM themes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var themes []map[string]interface{}
	for rows.Next() {
		var id, name string
		var isPremium bool
		var colorsJSON []byte
		if err := rows.Scan(&id, &name, &isPremium, &colorsJSON); err != nil {
			return nil, err
		}
		colors := map[string]string{}
		json.Unmarshal(colorsJSON, &colors)
		themes = append(themes, map[string]interface{}{
			"id": id, "name": name, "is_premium": isPremium, "colors": colors,
		})
	}
	return themes, nil
}

func (r *ThemeRepository) Create(name, authorID string, colors map[string]string) error {
	colorsJSON, _ := json.Marshal(colors)
	_, err := r.db.Exec(`INSERT INTO themes (name, author_id, colors) VALUES ($1, $2, $3)`, name, nullable(authorID), colorsJSON)
	return err
}

type WallpaperRepository struct {
	db *sql.DB
}

func NewWallpaperRepository(db *sql.DB) *WallpaperRepository {
	return &WallpaperRepository{db: db}
}

func (r *WallpaperRepository) List() ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`SELECT id, name, url, preview_url, is_premium, category FROM wallpapers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wallpapers []map[string]interface{}
	for rows.Next() {
		var id, name, url, previewURL, category string
		var isPremium bool
		if err := rows.Scan(&id, &name, &url, &previewURL, &isPremium, &category); err != nil {
			return nil, err
		}
		wallpapers = append(wallpapers, map[string]interface{}{
			"id": id, "name": name, "url": url, "preview_url": previewURL,
			"is_premium": isPremium, "category": category,
		})
	}
	return wallpapers, nil
}
