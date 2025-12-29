package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"pro-banana-ai-bot/internal/gemini"
	"pro-banana-ai-bot/internal/preview"
)

const previewCallbackPrefix = "pv"

func (h *Handler) startPreviewWizard(chatID int64, userID int64, args string, cover bool) error {
	defaults := preview.Options{
		Mode:          "grid",
		GridPreset:    "3x3",
		VerticalCount: "4",
	}
	if cover {
		defaults.GridPreset = "1x1"
		defaults.AspectRatio = "1:1"
		defaults.VisualStyle = "high_key_clean"
	}

	opts := preview.ParseArgs(args, defaults)
	st := h.preview.Update(chatID, userID, func(st *preview.UIState) {
		st.LastPhotoFileID = ""
		st.AwaitingCustom = false
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
		for i := 0; i < 9; i++ {
			st.SelectedFrames[i] = true
		}
		st.LastSelectedOrder = []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
		st.Menu = "main"
		st.AwaitingPhoto = true
	})

	msgID, err := h.tg.SendTextWithKeyboard(chatID, previewUIText(st), previewUIKeyboard(userID, st))
	if err != nil {
		return err
	}
	h.preview.Update(chatID, userID, func(st *preview.UIState) { st.MessageID = msgID })
	return nil
}

func (h *Handler) handleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) error {
	if q == nil || q.Message == nil {
		return nil
	}
	data := strings.TrimSpace(q.Data)
	if !strings.HasPrefix(data, previewCallbackPrefix+":") {
		return nil
	}

	parts := strings.Split(data, ":")
	if len(parts) < 3 {
		return nil
	}

	ownerID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil
	}
	if ownerID != q.From.ID {
		_ = h.tg.AnswerCallback(q.ID, "Bu menyu siz uchun emas.", true)
		return nil
	}

	action := parts[2]
	args := parts[3:]
	chatID := q.Message.Chat.ID
	msgID := q.Message.MessageID

	updated := h.preview.Update(chatID, ownerID, func(st *preview.UIState) {
		st.MessageID = msgID

		switch action {
		case "menu":
			if len(args) >= 1 {
				st.Menu = args[0]
			}
		case "mode":
			if len(args) >= 1 {
				if args[0] == "vertical" {
					st.Mode = "vertical"
				} else {
					st.Mode = "grid"
				}
				st.Menu = "main"
			}
		case "preset":
			if len(args) >= 2 {
				switch args[0] {
				case "grid":
					st.Mode = "grid"
					st.GridPreset = args[1]
				case "vertical":
					st.Mode = "vertical"
					st.VerticalCount = args[1]
				}
				st.Menu = "main"
			}
		case "cat":
			if len(args) >= 1 {
				if args[0] == "auto" {
					st.ProductType = ""
				} else {
					st.ProductType = args[0]
				}
				st.Menu = "main"
			}
		case "style":
			if len(args) >= 1 {
				if args[0] == "default" {
					st.VisualStyle = ""
				} else {
					st.VisualStyle = args[0]
				}
				st.Menu = "main"
			}
		case "human":
			st.HumanUsage = !st.HumanUsage
			st.Menu = "main"
		case "frame":
			if len(args) >= 1 {
				if idx, err := strconv.Atoi(args[0]); err == nil {
					st.ToggleFrame(idx)
				}
			}
			st.Menu = "frames"
		case "frames_reset":
			for i := 0; i < 9; i++ {
				st.SelectedFrames[i] = true
			}
			st.LastSelectedOrder = []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
			st.Menu = "frames"
		case "note":
			st.AwaitingCustom = true
			st.Menu = "main"
		case "await_photo":
			st.AwaitingPhoto = true
			st.Menu = "main"
		case "reset":
			lastPhoto := st.LastPhotoFileID
			msgID := st.MessageID
			*st = preview.UIState{}
			st.Mode = "grid"
			st.GridPreset = "3x3"
			st.VerticalCount = "4"
			st.AspectRatio = ""
			st.ProductType = ""
			st.VisualStyle = ""
			st.HumanUsage = false
			st.Custom = ""
			for i := 0; i < 9; i++ {
				st.SelectedFrames[i] = true
			}
			st.LastSelectedOrder = []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
			st.LastPhotoFileID = lastPhoto
			st.MessageID = msgID
			st.AwaitingPhoto = true
			st.Menu = "main"
		case "close":
			st.AwaitingCustom = false
			st.AwaitingPhoto = false
			st.Menu = "main"
		}
	})

	switch action {
	case "note":
		_ = h.tg.AnswerCallback(q.ID, "Note yuboring (bekor qilish: /cancel).", false)
		_ = h.tg.SendText(chatID, "📝 Qo'shimcha note yuboring (bekor qilish: /cancel).")
	case "prompt":
		_ = h.tg.AnswerCallback(q.ID, "Prompt yuborilyapti…", false)
		st := h.preview.Get(chatID, ownerID)
		prompt, _ := preview.BuildPrompt(st.PromptOptions())
		_ = h.tg.SendText(chatID, prompt)
	case "generate":
		_ = h.tg.AnswerCallback(q.ID, "Generating…", false)
		if strings.TrimSpace(updated.LastPhotoFileID) == "" {
			h.preview.Update(chatID, ownerID, func(st *preview.UIState) { st.AwaitingPhoto = true })
			_ = h.tg.SendText(chatID, "📷 Mahsulot rasmini yuboring.")
		} else {
			if err := h.generateFromPreviewState(ctx, chatID, ownerID, q.From.UserName, updated.LastPhotoFileID); err != nil {
				return err
			}
		}
	default:
		_ = h.tg.AnswerCallback(q.ID, "OK", false)
	}

	return h.renderPreviewUI(chatID, ownerID, msgID, true)
}

func (h *Handler) renderPreviewUI(chatID int64, userID int64, messageID int, edit bool) error {
	st := h.preview.Get(chatID, userID)
	if messageID == 0 {
		messageID = st.MessageID
	}

	text := previewUIText(st)
	kb := previewUIKeyboard(userID, st)

	if edit && messageID != 0 {
		if err := h.tg.EditTextWithKeyboard(chatID, messageID, text, kb); err == nil {
			return nil
		}
	}

	msgID, err := h.tg.SendTextWithKeyboard(chatID, text, kb)
	if err != nil {
		return err
	}
	h.preview.Update(chatID, userID, func(st *preview.UIState) { st.MessageID = msgID })
	return nil
}

func (h *Handler) generateFromPreviewState(ctx context.Context, chatID int64, userID int64, username string, fileID string) error {
	st := h.preview.Get(chatID, userID)
	opts := st.PromptOptions()
	prompt, out := preview.BuildPrompt(opts)

	h.tg.SendTyping(chatID)
	_ = h.tg.SendText(chatID, fmt.Sprintf("🎨 %d ta preview tayyorlanmoqda, biroz kuting...", out.Count))

	base64Data, mimeType, err := h.tg.DownloadFileBase64(ctx, fileID)
	if err != nil {
		h.logger.Error("preview photo download failed", "err", err)
		return h.tg.SendText(chatID, "❌ Rasmni yuklashda xatolik yuz berdi.")
	}

	resp, err := h.gem.Chat(ctx, nil, prompt, []gemini.ImageInput{{DataBase64: base64Data, MimeType: mimeType}}, gemini.ChatOptions{WantImage: true, AspectRatio: out.AspectRatio})
	if err != nil {
		h.logger.Error("preview generation failed", "err", err)
		return h.tg.SendText(chatID, "❌ Preview yaratishda xatolik yuz berdi. Qayta urinib ko'ring.")
	}

	if len(resp.Images) == 0 {
		return h.tg.SendText(chatID, "❌ Preview rasm(lar)i chiqarmadi. Boshqa rasm yuboring yoki tavsifni qisqartiring.")
	}

	h.preview.Update(chatID, userID, func(st *preview.UIState) {
		st.LastPhotoFileID = fileID
		st.AwaitingPhoto = false
		st.Menu = "main"
	})

	caption := fmt.Sprintf("✅ Tayyor! preview (%d ta)", len(resp.Images))
	if opts.VisualStyle != "" {
		caption += ", style=" + opts.VisualStyle
	}
	if opts.ProductType != "" {
		caption += ", cat=" + opts.ProductType
	}

	_ = username // reserved for future per-user history if needed
	return h.sendGeminiResponse(chatID, gemini.Response{
		Text:   caption,
		Images: resp.Images,
	}, true)
}

func previewUIText(st preview.UIState) string {
	opts := st.PromptOptions()
	out := preview.ResolveOutputPreset(opts)

	mode := "Grid"
	preset := st.GridPreset
	if strings.ToLower(st.Mode) == "vertical" {
		mode = "Vertical"
		preset = "v" + st.VerticalCount
	}

	category := "Auto"
	for _, o := range preview.ProductCategories() {
		if o.Key == st.ProductType {
			category = o.Name
			break
		}
	}

	style := "Default"
	if st.VisualStyle != "" {
		for _, o := range preview.VisualStyles() {
			if o.Key == st.VisualStyle {
				style = o.Name
				break
			}
		}
	}

	selected := countSelectedFrames(st)

	var b strings.Builder
	b.WriteString("🧩 Product Shot Preview\n\n")
	b.WriteString(fmt.Sprintf("Mode: %s (%s)\n", mode, preset))
	b.WriteString(fmt.Sprintf("Images: %d, AR: %s\n", out.Count, out.AspectRatio))
	b.WriteString(fmt.Sprintf("Category: %s\n", category))
	b.WriteString(fmt.Sprintf("Style: %s\n", style))
	b.WriteString(fmt.Sprintf("Human usage: %s\n", yesNo(st.HumanUsage)))
	b.WriteString(fmt.Sprintf("Frames: %d/%d\n", selected, out.Count))
	if strings.TrimSpace(st.Custom) != "" {
		b.WriteString("Note: " + truncateLine(st.Custom, 80) + "\n")
	}
	if strings.TrimSpace(st.LastPhotoFileID) == "" {
		b.WriteString("Photo: (none)\n")
	} else {
		b.WriteString("Photo: saved ✅\n")
	}
	if st.AwaitingCustom {
		b.WriteString("\n📝 Endi note yuboring (bekor qilish: /cancel).\n")
	} else if st.AwaitingPhoto {
		b.WriteString("\n📷 Endi mahsulot rasmini yuboring.\n")
	} else if strings.TrimSpace(st.LastPhotoFileID) != "" {
		b.WriteString("\n🎨 Endi `Generate` tugmasini bosing.\n")
		b.WriteString("📷 Rasmni almashtirish: yangi rasmni shunchaki yuboring (caption bo'sh) yoki `Photo`.\n")
	}

	if st.Menu == "frames" {
		b.WriteString("\nFrames list:\n")
		frames := preview.FrameTemplates()
		for i, f := range frames {
			if i >= 9 {
				break
			}
			b.WriteString(fmt.Sprintf("%d) %s\n", i+1, f.Title))
		}
	}

	return strings.TrimSpace(b.String())
}

func previewUIKeyboard(ownerID int64, st preview.UIState) tgbotapi.InlineKeyboardMarkup {
	switch st.Menu {
	case "category":
		return categoryKeyboard(ownerID, st)
	case "style":
		return styleKeyboard(ownerID, st)
	case "frames":
		return framesKeyboard(ownerID, st)
	default:
		return mainKeyboard(ownerID, st)
	}
}

func mainKeyboard(ownerID int64, st preview.UIState) tgbotapi.InlineKeyboardMarkup {
	if strings.TrimSpace(st.LastPhotoFileID) == "" {
		return tgbotapi.NewInlineKeyboardMarkup(
			[]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("📷 Send photo", cb(ownerID, "await_photo")),
				tgbotapi.NewInlineKeyboardButtonData("Close", cb(ownerID, "close")),
			},
			[]tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData("Reset", cb(ownerID, "reset")),
			},
		)
	}

	gridText := "Grid"
	verticalText := "Vertical"
	if strings.ToLower(st.Mode) != "vertical" {
		gridText = "✅ Grid"
	} else {
		verticalText = "✅ Vertical"
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData(gridText, cb(ownerID, "mode", "grid")),
			tgbotapi.NewInlineKeyboardButtonData(verticalText, cb(ownerID, "mode", "vertical")),
		},
	}

	if strings.ToLower(st.Mode) == "vertical" {
		var presetRow []tgbotapi.InlineKeyboardButton
		for _, v := range []string{"1", "2", "3", "4"} {
			label := "v" + v
			if st.VerticalCount == v {
				label = "✅ " + label
			}
			presetRow = append(presetRow, tgbotapi.NewInlineKeyboardButtonData(label, cb(ownerID, "preset", "vertical", v)))
		}
		rows = append(rows, presetRow)
	} else {
		var presetRow []tgbotapi.InlineKeyboardButton
		for _, g := range []string{"1x1", "2x2", "3x2", "3x3"} {
			label := g
			if st.GridPreset == g {
				label = "✅ " + label
			}
			presetRow = append(presetRow, tgbotapi.NewInlineKeyboardButtonData(label, cb(ownerID, "preset", "grid", g)))
		}
		rows = append(rows, presetRow)
	}

	rows = append(rows,
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Category", cb(ownerID, "menu", "category")),
			tgbotapi.NewInlineKeyboardButtonData("Style", cb(ownerID, "menu", "style")),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Human: "+onOff(st.HumanUsage), cb(ownerID, "human")),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Frames (%d)", countSelectedFrames(st)), cb(ownerID, "menu", "frames")),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Note", cb(ownerID, "note")),
			tgbotapi.NewInlineKeyboardButtonData("📄 Prompt", cb(ownerID, "prompt")),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("📷 Photo", cb(ownerID, "await_photo")),
			tgbotapi.NewInlineKeyboardButtonData("🎨 Generate", cb(ownerID, "generate")),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Reset", cb(ownerID, "reset")),
			tgbotapi.NewInlineKeyboardButtonData("Close", cb(ownerID, "close")),
		},
	)

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func categoryKeyboard(ownerID int64, st preview.UIState) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	var row []tgbotapi.InlineKeyboardButton
	for _, opt := range preview.ProductCategories() {
		key := opt.Key
		cbKey := key
		if cbKey == "" {
			cbKey = "auto"
		}

		label := opt.Name
		if key == st.ProductType {
			label = "✅ " + label
		}

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cb(ownerID, "cat", cbKey)))
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Back", cb(ownerID, "menu", "main")),
	})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func styleKeyboard(ownerID int64, st preview.UIState) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	for _, opt := range preview.VisualStyles() {
		key := opt.Key
		cbKey := key
		if cbKey == "" {
			cbKey = "default"
		}

		label := opt.Name
		if key == st.VisualStyle || (key == "" && st.VisualStyle == "") {
			label = "✅ " + label
		}

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cb(ownerID, "style", cbKey)))
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅ Back", cb(ownerID, "menu", "main")),
	})

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func framesKeyboard(ownerID int64, st preview.UIState) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for r := 0; r < 3; r++ {
		var row []tgbotapi.InlineKeyboardButton
		for c := 0; c < 3; c++ {
			idx := r*3 + c
			label := fmt.Sprintf("%d", idx+1)
			if st.SelectedFrames[idx] {
				label = "✅ " + label
			} else {
				label = "⬜ " + label
			}
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cb(ownerID, "frame", strconv.Itoa(idx))))
		}
		rows = append(rows, row)
	}

	rows = append(rows,
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("Reset frames", cb(ownerID, "frames_reset")),
			tgbotapi.NewInlineKeyboardButtonData("⬅ Back", cb(ownerID, "menu", "main")),
		},
	)
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func cb(ownerID int64, parts ...string) string {
	return fmt.Sprintf("%s:%d:%s", previewCallbackPrefix, ownerID, strings.Join(parts, ":"))
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func yesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func truncateLine(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimSpace(string(runes[:max])) + "…"
}

func countSelectedFrames(st preview.UIState) int {
	n := 0
	for i := 0; i < 9; i++ {
		if st.SelectedFrames[i] {
			n++
		}
	}
	return n
}
