package telegram

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Options struct {
	Token      string
	HTTPClient *http.Client
	Logger     *slog.Logger
	Debug      bool
}

type Client struct {
	bot        *tgbotapi.BotAPI
	httpClient *http.Client
	logger     *slog.Logger
}

func New(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, errors.New("telegram token is empty")
	}
	if opts.HTTPClient == nil {
		return nil, errors.New("http client is nil")
	}

	bot, err := tgbotapi.NewBotAPIWithClient(opts.Token, tgbotapi.APIEndpoint, opts.HTTPClient)
	if err != nil {
		return nil, err
	}
	bot.Debug = opts.Debug

	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Client{
		bot:        bot,
		httpClient: opts.HTTPClient,
		logger:     logger,
	}, nil
}

func (c *Client) Username() string {
	return c.bot.Self.UserName
}

type Update = tgbotapi.Update

type UpdatesOptions struct {
	Timeout time.Duration
}

func (c *Client) Updates(opts UpdatesOptions) tgbotapi.UpdatesChannel {
	u := tgbotapi.NewUpdate(0)
	if opts.Timeout > 0 {
		u.Timeout = int(opts.Timeout.Seconds())
	} else {
		u.Timeout = 30
	}
	return c.bot.GetUpdatesChan(u)
}

func (c *Client) StopUpdates() {
	c.bot.StopReceivingUpdates()
}

func (c *Client) SendTyping(chatID int64) {
	_, _ = c.bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
}

func (c *Client) SendText(chatID int64, text string) error {
	parts := splitByBytes(text, 4096)
	for _, p := range parts {
		msg := tgbotapi.NewMessage(chatID, p)
		if _, err := c.bot.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) SendPhotoDataURL(chatID int64, dataURL string, caption string) error {
	mimeType, base64Data, err := parseDataURL(dataURL)
	if err != nil {
		return err
	}

	bytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	name := "image.jpg"
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		name = "image" + exts[0]
	}

	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{
		Name:  name,
		Bytes: bytes,
	})
	if caption != "" {
		photo.Caption = truncateByBytes(caption, 1024)
	}

	_, err = c.bot.Send(photo)
	return err
}

func (c *Client) DownloadFileBase64(ctx context.Context, fileID string) (string, string, error) {
	fileURL, err := c.bot.GetFileDirectURL(fileID)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("telegram file download %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	mimeType := strings.TrimSpace(resp.Header.Get("content-type"))
	if strings.Contains(mimeType, ";") {
		mimeType = strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(bytes)
	}
	if strings.Contains(mimeType, ";") {
		mimeType = strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = "image/jpeg"
	}

	return base64.StdEncoding.EncodeToString(bytes), mimeType, nil
}

func parseDataURL(value string) (mimeType string, base64Data string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", errors.New("empty data url")
	}

	const prefix = "data:"
	if !strings.HasPrefix(value, prefix) {
		return "image/jpeg", value, nil
	}

	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid data url")
	}

	meta := strings.TrimPrefix(parts[0], prefix)
	metaParts := strings.Split(meta, ";")
	mimeType = strings.TrimSpace(metaParts[0])
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return mimeType, parts[1], nil
}

func splitByBytes(text string, maxBytes int) []string {
	if len([]byte(text)) <= maxBytes || maxBytes <= 0 {
		return []string{text}
	}

	var out []string
	var buf strings.Builder
	buf.Grow(maxBytes)

	for _, r := range text {
		runeBytes := utf8.RuneLen(r)
		if runeBytes < 0 {
			runeBytes = len([]byte(string(r)))
		}

		if buf.Len() > 0 && buf.Len()+runeBytes > maxBytes {
			out = append(out, buf.String())
			buf.Reset()
		}
		buf.WriteRune(r)
	}

	if buf.Len() > 0 {
		out = append(out, buf.String())
	}

	return out
}

func truncateByBytes(text string, maxBytes int) string {
	if len([]byte(text)) <= maxBytes || maxBytes <= 0 {
		return text
	}

	var buf strings.Builder
	buf.Grow(maxBytes)
	for _, r := range text {
		runeBytes := utf8.RuneLen(r)
		if runeBytes < 0 {
			runeBytes = len([]byte(string(r)))
		}

		if buf.Len()+runeBytes > maxBytes {
			break
		}
		buf.WriteRune(r)
	}
	return buf.String()
}
