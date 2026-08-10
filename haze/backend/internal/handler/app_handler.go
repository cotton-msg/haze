package handler

import (
	"encoding/json"
	"net/http"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type SettingsHandler struct {
	settingsRepo  *repository.UserSettingsRepository
	themeRepo     *repository.ThemeRepository
	wallpaperRepo *repository.WallpaperRepository
}

func NewSettingsHandler(sr *repository.UserSettingsRepository, tr *repository.ThemeRepository, wr *repository.WallpaperRepository) *SettingsHandler {
	return &SettingsHandler{settingsRepo: sr, themeRepo: tr, wallpaperRepo: wr}
}

func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	settings, _ := h.settingsRepo.Get(claims.UserID)
	utils.SuccessResponse(w, settings)
}

func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	var req struct {
		ThemeID      string                 `json:"theme_id"`
		WallpaperURL string                 `json:"wallpaper_url"`
		NotifSounds  map[string]interface{} `json:"notification_sounds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	h.settingsRepo.Upsert(claims.UserID, req.ThemeID, req.WallpaperURL, req.NotifSounds)
	utils.SuccessResponse(w, map[string]string{"status": "saved"})
}

func (h *SettingsHandler) ListThemes(w http.ResponseWriter, r *http.Request) {
	themes, err := h.themeRepo.List()
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, themes)
}

func (h *SettingsHandler) ListWallpapers(w http.ResponseWriter, r *http.Request) {
	wallpapers, err := h.wallpaperRepo.List()
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, wallpapers)
}

type FolderHandler struct {
	folderRepo *repository.FolderRepository
}

func NewFolderHandler(fr *repository.FolderRepository) *FolderHandler {
	return &FolderHandler{folderRepo: fr}
}

func (h *FolderHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	folders, _ := h.folderRepo.FindByUserID(claims.UserID)
	utils.SuccessResponse(w, folders)
}

func (h *FolderHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	var req struct {
		Name  string `json:"name"`
		Icon  string `json:"icon"`
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	folder := &models.ChatFolder{
		UserID: claims.UserID, Name: req.Name,
		Icon: req.Icon, Emoji: req.Emoji,
	}
	if err := h.folderRepo.Create(folder); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, folder)
}

func (h *FolderHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	folderID := chi.URLParam(r, "id")

	var req struct {
		Name     string `json:"name"`
		Icon     string `json:"icon"`
		Emoji    string `json:"emoji"`
		Position int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	folder := &models.ChatFolder{
		ID: folderID, UserID: claims.UserID,
		Name: req.Name, Icon: req.Icon, Emoji: req.Emoji, Position: req.Position,
	}
	if err := h.folderRepo.Update(folder); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, folder)
}

func (h *FolderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	if err := h.folderRepo.Delete(folderID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "deleted"})
}

func (h *FolderHandler) AddChat(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	var req struct {
		ChatID string `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "chat_id is required")
		return
	}
	if err := h.folderRepo.AddChat(folderID, req.ChatID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "added"})
}

func (h *FolderHandler) RemoveChat(w http.ResponseWriter, r *http.Request) {
	folderID := chi.URLParam(r, "id")
	chatID := chi.URLParam(r, "chatId")
	if err := h.folderRepo.RemoveChat(folderID, chatID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "removed"})
}

type BotHandler struct {
	botRepo  *repository.BotRepository
	cmdRepo  *repository.BotCommandRepository
	userRepo *repository.UserRepository
}

func NewBotHandler(br *repository.BotRepository, cr *repository.BotCommandRepository, ur *repository.UserRepository) *BotHandler {
	return &BotHandler{botRepo: br, cmdRepo: cr, userRepo: ur}
}

func (h *BotHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	var req struct {
		Username    string `json:"username"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Username == "" || req.Name == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "username and name are required")
		return
	}
	token := "haze_" + claims.UserID[:8] + "_" + req.Username + "_token"
	id, err := h.botRepo.Create(claims.UserID, token, req.Username, req.Name, req.Description)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"id": id, "token": token, "username": req.Username})
}

func (h *BotHandler) List(w http.ResponseWriter, r *http.Request) {
	bots, _ := h.botRepo.List()
	utils.SuccessResponse(w, bots)
}

// SetWebhook задаёт URL, куда бот получает обновления о новых сообщениях.
func (h *BotHandler) SetWebhook(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	botID := chi.URLParam(r, "id")
	bot, err := h.botRepo.FindByID(botID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "bot not found")
		return
	}
	if bot["owner_id"] != claims.UserID {
		utils.ErrorResponse(w, http.StatusForbidden, "not your bot")
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.botRepo.SetWebhook(botID, req.URL); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "ok"})
}

// SendMessage отправляет сообщение от имени бота в чат.
// Аутентификация — заголовок X-Bot-Token.
func (h *BotHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	bot, err := h.botRepo.FindByToken(r.Header.Get("X-Bot-Token"))
	if err != nil {
		utils.ErrorResponse(w, http.StatusUnauthorized, "invalid bot token")
		return
	}
	var req struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" || req.Text == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "chat_id and text are required")
		return
	}
	msg, err := h.botClient.SendMessage(bot["id"].(string), req.ChatID, req.Text)
	if err != nil {
		utils.ErrorResponse(w, http.StatusBadGateway, err.Error())
		return
	}
	utils.SuccessResponse(w, msg)
}

// SetCommands заменяет список команд бота.
func (h *BotHandler) SetCommands(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	botID := chi.URLParam(r, "id")
	bot, err := h.botRepo.FindByID(botID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "bot not found")
		return
	}
	if bot["owner_id"] != claims.UserID {
		utils.ErrorResponse(w, http.StatusForbidden, "not your bot")
		return
	}
	var req struct {
		Commands []map[string]string `json:"commands"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.cmdRepo.SetCommands(botID, req.Commands); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "ok"})
}

// GetCommands возвращает список команд бота.
func (h *BotHandler) GetCommands(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "id")
	commands, err := h.cmdRepo.GetCommands(botID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, commands)
}

type AdminHandler struct {
	badgeRepo *repository.BadgeRepository
	userRepo  *repository.UserRepository
	botRepo   *repository.BotRepository
	statsRepo *repository.StatsRepository
}

func NewAdminHandler(br *repository.BadgeRepository, ur *repository.UserRepository, botr *repository.BotRepository, sr *repository.StatsRepository) *AdminHandler {
	return &AdminHandler{badgeRepo: br, userRepo: ur, botRepo: botr, statsRepo: sr}
}

func (h *AdminHandler) AssignBadge(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if claims.Role != "owner" && claims.Role != "developer" && claims.Role != "admin" {
		utils.ErrorResponse(w, http.StatusForbidden, "only admins can assign badges")
		return
	}
	var req struct {
		UserID    string `json:"user_id"`
		BadgeType string `json:"badge_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	h.badgeRepo.Assign(req.UserID, req.BadgeType, claims.UserID)
	utils.SuccessResponse(w, map[string]string{"status": "assigned"})
}

func (h *AdminHandler) RemoveBadge(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if claims.Role != "owner" && claims.Role != "developer" && claims.Role != "admin" {
		utils.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}
	userID := chi.URLParam(r, "userId")
	badgeType := chi.URLParam(r, "badgeType")
	h.badgeRepo.Remove(userID, badgeType)
	utils.SuccessResponse(w, map[string]string{"status": "removed"})
}

func (h *AdminHandler) GetBadges(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	badges, _ := h.badgeRepo.GetByUser(userID)
	utils.SuccessResponse(w, badges)
}

func (h *AdminHandler) GetAllBadges(w http.ResponseWriter, r *http.Request) {
	badges, _ := h.badgeRepo.GetAll()
	utils.SuccessResponse(w, badges)
}

func (h *AdminHandler) ChangeRole(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if claims.Role != "owner" {
		utils.ErrorResponse(w, http.StatusForbidden, "only owner can change roles")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	if urlID := chi.URLParam(r, "userId"); urlID != "" {
		req.UserID = urlID
	}
	if req.UserID == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "user_id is required")
		return
	}

	roleLevel := map[string]int{"user": 0, "moderator": 1, "admin": 2, "developer": 3, "owner": 4}
	level, ok := roleLevel[req.Role]
	if !ok {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid role, allowed: user, moderator, admin, developer, owner")
		return
	}

	target, err := h.userRepo.FindByID(req.UserID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, "user not found")
		return
	}

	actorLevel := roleLevel[claims.Role]
	if actorLevel <= roleLevel[target.Role] {
		utils.ErrorResponse(w, http.StatusForbidden, "cannot change role of a user with equal or higher role")
		return
	}
	if level >= actorLevel {
		utils.ErrorResponse(w, http.StatusForbidden, "cannot assign a role equal or higher than your own")
		return
	}

	if err := h.userRepo.UpdateRole(req.UserID, req.Role); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "role updated", "role": req.Role})
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsRepo.GetDashboard()
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, stats)
}
