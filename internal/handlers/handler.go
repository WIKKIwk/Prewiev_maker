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
	"pro-banana-ai-bot/internal/session"
	"pro-banana-ai-bot/internal/telegram"
)

type Options struct {
	Telegram *telegram.Client
	Gemini   *gemini.Client
	Sessions *session.Store
	Logger   *slog.Logger
}

type Handler struct {
	tg         *telegram.Client
	gem        *gemini.Client
	sessions   *session.Store
	logger     *slog.Logger
	aggregator *mediagroup.Aggregator
}

func New(opts Options) *Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		tg:       opts.Telegram,
		gem:      opts.Gemini,
		sessions: opts.Sessions,
		logger:   logger,
	}
}

func (h *Handler) SetMediaGroupAggregator(ag *mediagroup.Aggregator) {
	h.aggregator = ag
}

func (h *Handler) HandleUpdate(ctx context.Context, update telegram.Update) error {
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
				"/image <tavsif> - Rasm yaratish\n"+
				"/clear - Suhbat tarixini tozalash",
		)
	case "help":
		return h.tg.SendText(chatID,
			"🍌 Yordam\n\n"+
				"Matn yuboring — javob beraman.\n"+
				"Rasm yuboring — tahlil/tahrir qilaman.\n"+
				"/image <tavsif> — rasm yaratish.\n"+
				"/clear — suhbat tarixini tozalash.",
		)
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

	caption := strings.TrimSpace(msg.Caption)
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
