package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/sync/errgroup"

	"pro-banana-ai-bot/internal/gemini"
	"pro-banana-ai-bot/internal/mediagroup"
	"pro-banana-ai-bot/internal/preview"
	"pro-banana-ai-bot/internal/session"
	"pro-banana-ai-bot/internal/telegram"
)

type Options struct {
	Telegram *telegram.Client
	Gemini   *gemini.Client
	Sessions *session.Store
	Logger   *slog.Logger
	Preview  *preview.Store
}

type Handler struct {
	tg         *telegram.Client
	gem        *gemini.Client
	sessions   *session.Store
	logger     *slog.Logger
	aggregator *mediagroup.Aggregator
	preview    *preview.Store
}

func New(opts Options) *Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	pv := opts.Preview
	if pv == nil {
		pv = preview.NewStore()
	}

	return &Handler{
		tg:       opts.Telegram,
		gem:      opts.Gemini,
		sessions: opts.Sessions,
		logger:   logger,
		preview:  pv,
	}
}

func (h *Handler) SetMediaGroupAggregator(ag *mediagroup.Aggregator) {
	h.aggregator = ag
}

func (h *Handler) HandleUpdate(ctx context.Context, update telegram.Update) error {
	if update.CallbackQuery != nil {
		return h.handleCallback(ctx, update.CallbackQuery)
	}

	if update.Message == nil {
		return nil
	}

	msg := update.Message
	chatID := msg.Chat.ID
	userID := msg.From.ID
	username := msg.From.UserName

	if msg.IsCommand() {
		return h.handleCommand(ctx, chatID, userID, username, msg)
	}

	if len(msg.Photo) > 0 {
		return h.handlePhoto(ctx, chatID, userID, username, msg)
	}

	if msg.Text != "" {
		return h.handleText(ctx, chatID, userID, username, msg.Text)
	}

	return nil
}

func (h *Handler) HandleMediaGroup(ctx context.Context, group mediagroup.Group) {
	caption := strings.TrimSpace(group.Caption)
	if caption == "" {
		caption = "Bu rasmlarni tahlil qiling"
	}

	if cmd, args, ok := parseLeadingCommand(caption); ok {
		switch cmd {
		case "preview", "cover":
			if err := h.handlePreview(ctx, group.ChatID, group.UserID, group.Username, cmd, args, group.FileIDs); err != nil {
				h.logger.Error("preview processing failed", "err", err)
			}
			return
		}
	}
	if err := h.processPhotos(ctx, group.ChatID, group.UserID, group.Username, caption, group.FileIDs); err != nil {
		h.logger.Error("media group processing failed", "err", err)
	}
}

func (h *Handler) handleCommand(ctx context.Context, chatID int64, userID int64, username string, msg *tgbotapi.Message) error {
	switch msg.Command() {
	case "start":
		return h.tg.SendText(chatID,
			"🍌 Pro Banana AI Bot\n\n"+
				"Assalomu alaykum! Menga xabar yoki rasm yuboring.\n\n"+
				"Buyruqlar:\n"+
				"/start - Botni ishga tushirish\n"+
				"/help - Yordam\n"+
				"/preview - Marketplace preview (wizard)\n"+
				"/cover - 1 ta cover (wizard)\n"+
				"/cancel - Preview wizardni bekor qilish\n"+
				"/image <tavsif> - Rasm yaratish\n"+
				"/clear - Suhbat tarixini tozalash",
		)
	case "help":
		return h.tg.SendText(chatID,
			"🍌 Yordam\n\n"+
				"Matn yuboring — javob beraman.\n"+
				"Rasm yuboring — tahlil/tahrir qilaman.\n"+
				"/preview — marketplace uchun pro preview (web'dagidek presetlar bilan).\n"+
				"/cover — marketplace cover (1 ta rasm).\n"+
				"/cancel — preview wizardni bekor qilish.\n"+
				"/image <tavsif> — rasm yaratish.\n"+
				"/clear — suhbat tarixini tozalash.",
		)
	case "preview":
		return h.startPreviewWizard(chatID, userID, msg.CommandArguments(), false)
	case "cover":
		return h.startPreviewWizard(chatID, userID, msg.CommandArguments(), true)
	case "cancel":
		h.preview.Update(chatID, userID, func(st *preview.UIState) {
			st.AwaitingCustom = false
			st.AwaitingPhoto = false
			st.Menu = "main"
		})
		return h.tg.SendText(chatID, "✅ Bekor qilindi.")
	case "clear":
		h.sessions.Clear(userID)
		return h.tg.SendText(chatID, "✅ Suhbat tarixi tozalandi!")
	case "image":
		prompt := strings.TrimSpace(msg.CommandArguments())
		if prompt == "" {
			return h.tg.SendText(chatID, "❌ Iltimos, rasm tavsifini kiriting!\nMisol: /image banana in space")
		}

		h.tg.SendTyping(chatID)
		_ = h.tg.SendText(chatID, "🎨 Rasm yaratilmoqda, biroz kuting...")

		images, err := h.gem.GenerateImage(ctx, prompt)
		if err != nil {
			h.logger.Error("image generation failed", "err", err)
			return h.tg.SendText(chatID, "❌ Rasm yaratishda xatolik yuz berdi. Qayta urinib ko'ring.")
		}

		if len(images) == 0 {
			return h.tg.SendText(chatID, "❌ Rasm yaratishda xatolik yuz berdi. Qayta urinib ko'ring.")
		}

		caption := fmt.Sprintf("✅ Tayyor! Rasm: %q", prompt)
		for i, img := range images {
			sendCaption := ""
			if i == 0 {
				sendCaption = caption
			}
			if err := h.tg.SendPhotoDataURL(chatID, img, sendCaption); err != nil {
				return err
			}
		}
		return nil
	default:
		return h.tg.SendText(chatID, "❌ Noma'lum buyruq. /help ni ishlating.")
	}
}

func (h *Handler) handleText(ctx context.Context, chatID int64, userID int64, username string, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	if st := h.preview.Get(chatID, userID); st.AwaitingCustom {
		updated := h.preview.Update(chatID, userID, func(st *preview.UIState) {
			st.Custom = text
			st.AwaitingCustom = false
			st.Menu = "main"
		})
		_ = h.tg.SendText(chatID, "✅ Note saqlandi.")
		if updated.MessageID != 0 {
			if err := h.renderPreviewUI(chatID, userID, updated.MessageID, true); err == nil {
				return nil
			}
		}
		return h.renderPreviewUI(chatID, userID, 0, false)
	}

	h.tg.SendTyping(chatID)

	history := h.sessions.Snapshot(userID, username)
	geminiHistory := toGeminiHistory(history)

	resp, err := h.gem.Chat(ctx, geminiHistory, text, nil, gemini.ChatOptions{})
	if err != nil {
		h.logger.Error("gemini chat failed", "err", err)
		return h.tg.SendText(chatID, "❌ Xatolik yuz berdi. Iltimos, qayta urinib ko'ring.")
	}

	h.sessions.Append(userID, username,
		session.HistoryMessage{Role: "user", Content: text},
		session.HistoryMessage{Role: "model", Content: resp.Text, ImageURLs: resp.Images},
	)

	return h.sendGeminiResponse(chatID, resp, false)
}

func (h *Handler) handlePhoto(ctx context.Context, chatID int64, userID int64, username string, msg *tgbotapi.Message) error {
	photo := msg.Photo[len(msg.Photo)-1]
	fileID := photo.FileID

	if msg.MediaGroupID != "" && h.aggregator != nil {
		h.aggregator.Add(mediagroup.Item{
			ChatID:       chatID,
			UserID:       userID,
			Username:     username,
			MediaGroupID: msg.MediaGroupID,
			Caption:      msg.Caption,
			FileID:       fileID,
		})
		return nil
	}

	rawCaption := strings.TrimSpace(msg.Caption)
	if cmd, args, ok := parseLeadingCommand(rawCaption); ok {
		switch cmd {
		case "preview", "cover":
			return h.handlePreview(ctx, chatID, userID, username, cmd, args, []string{fileID})
		}
	}

	if st := h.preview.Get(chatID, userID); st.AwaitingPhoto || (rawCaption == "" && st.MessageID != 0 && !st.AwaitingCustom) {
		updated := h.preview.Update(chatID, userID, func(st *preview.UIState) {
			st.LastPhotoFileID = fileID
			st.AwaitingPhoto = false
			st.AwaitingCustom = false
			st.Menu = "main"
		})
		_ = username
		return h.renderPreviewUI(chatID, userID, updated.MessageID, true)
	}

	caption := rawCaption
	if caption == "" {
		caption = "Bu rasmni tahlil qiling"
	}

	return h.processPhotos(ctx, chatID, userID, username, caption, []string{fileID})
}

func (h *Handler) processPhotos(ctx context.Context, chatID int64, userID int64, username, caption string, fileIDs []string) error {
	h.tg.SendTyping(chatID)

	type downloaded struct {
		Base64 string
		Mime   string
	}

	downloads := make([]downloaded, len(fileIDs))
	eg, egCtx := errgroup.WithContext(ctx)
	for i, fileID := range fileIDs {
		i := i
		fileID := fileID
		eg.Go(func() error {
			data, mimeType, err := h.tg.DownloadFileBase64(egCtx, fileID)
			if err != nil {
				return err
			}
			downloads[i] = downloaded{Base64: data, Mime: mimeType}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		h.logger.Error("photo download failed", "err", err)
		return h.tg.SendText(chatID, "❌ Rasmni yuklashda xatolik yuz berdi.")
	}

	images := make([]gemini.ImageInput, 0, len(downloads))
	imageURLs := make([]string, 0, len(downloads))
	for _, d := range downloads {
		images = append(images, gemini.ImageInput{DataBase64: d.Base64, MimeType: d.Mime})
		imageURLs = append(imageURLs, fmt.Sprintf("data:%s;base64,%s", d.Mime, d.Base64))
	}

	history := h.sessions.Snapshot(userID, username)
	geminiHistory := toGeminiHistory(history)

	wantImage := wantsImageOutput(caption, len(fileIDs))
	resp, err := h.gem.Chat(ctx, geminiHistory, caption, images, gemini.ChatOptions{WantImage: wantImage})
	if err != nil {
		h.logger.Error("gemini photo prompt failed", "err", err)
		return h.tg.SendText(chatID, "❌ Xatolik yuz berdi. Iltimos, qayta urinib ko'ring.")
	}

	h.sessions.Append(userID, username,
		session.HistoryMessage{Role: "user", Content: caption, ImageURLs: imageURLs},
		session.HistoryMessage{Role: "model", Content: resp.Text, ImageURLs: resp.Images},
	)

	return h.sendGeminiResponse(chatID, resp, wantImage)
}

func (h *Handler) sendGeminiResponse(chatID int64, resp gemini.Response, preferImage bool) error {
	if len(resp.Images) == 0 {
		if preferImage && looksLikeToolCall(resp.Text) {
			return h.tg.SendText(chatID, "❌ Rasmni tahrir qilib qaytara olmadim. Iltimos, rasm(lar)ni qayta yuboring yoki tavsifni qisqartiring.")
		}
		return h.tg.SendText(chatID, resp.Text)
	}

	caption := resp.Text
	for i, img := range resp.Images {
		sendCaption := ""
		if i == 0 {
			sendCaption = caption
		}
		if err := h.tg.SendPhotoDataURL(chatID, img, sendCaption); err != nil {
			return err
		}
	}
	return nil
}

func toGeminiHistory(history []session.HistoryMessage) []gemini.Message {
	out := make([]gemini.Message, 0, len(history))
	for _, m := range history {
		role := m.Role
		if role == "" {
			role = "user"
		}
		out = append(out, gemini.Message{
			Role:      role,
			Text:      m.Content,
			ImageURLs: m.ImageURLs,
		})
	}
	return out
}

func parseLeadingCommand(text string) (cmd string, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}

	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", "", false
	}

	token := fields[0]
	if !strings.HasPrefix(token, "/") {
		return "", "", false
	}

	token = strings.TrimPrefix(token, "/")
	if at := strings.IndexByte(token, '@'); at >= 0 {
		token = token[:at]
	}
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return "", "", false
	}

	return token, strings.TrimSpace(text[len(fields[0]):]), true
}

func (h *Handler) handlePreview(ctx context.Context, chatID int64, userID int64, username, cmd, args string, fileIDs []string) error {
	_ = ctx
	_ = username

	if len(fileIDs) == 0 {
		return h.tg.SendText(chatID, "❌ Rasm topilmadi.")
	}

	defaults := preview.Options{
		Mode:          "grid",
		GridPreset:    "3x3",
		VerticalCount: "4",
	}
	if cmd == "cover" {
		defaults.GridPreset = "1x1"
		defaults.AspectRatio = "1:1"
		defaults.VisualStyle = "high_key_clean"
	}

	opts := preview.ParseArgs(args, defaults)

	updated := h.preview.Update(chatID, userID, func(st *preview.UIState) {
		st.Mode = opts.Mode
		st.GridPreset = opts.GridPreset
		st.VerticalCount = opts.VerticalCount
		st.AspectRatio = opts.AspectRatio
		st.ProductType = opts.ProductType
		st.VisualStyle = opts.VisualStyle
		st.HumanUsage = opts.HumanUsage
		if strings.TrimSpace(opts.Custom) != "" {
			st.Custom = opts.Custom
		}
		st.LastPhotoFileID = fileIDs[0]
		st.AwaitingPhoto = false
		st.Menu = "main"
	})
	return h.renderPreviewUI(chatID, userID, updated.MessageID, true)
}
