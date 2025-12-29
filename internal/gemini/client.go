package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

const (
	modelText  = "gemini-3-pro-preview"
	modelImage = "gemini-2.5-flash-image"
)

const systemInstruction = `Siz "Pro Banana AI" asistentsiz.
Siz Gemini 3 Pro (matn) va Gemini 2.5 Flash (tasvir) modellaridan foydalanasiz.
Vazifalaringiz:
1. Foydalanuvchi bilan o'zbek tilida aqlli suhbat qurish.
2. Rasmlarni tahlil qilish va tahrirlash.
3. Murakkab savollarga aniq javob berish.`

type Options struct {
	APIKey     string
	BaseURL    string
	APIVersion string
	HTTPClient *http.Client
	Logger     *slog.Logger
}

type ChatOptions struct {
	WantImage bool
}

type Client struct {
	apiKey     string
	baseURL    string
	apiVersion string
	httpClient *http.Client
	logger     *slog.Logger
}

func New(opts Options) *Client {
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	apiVersion := strings.TrimSpace(opts.APIVersion)
	if apiVersion == "" {
		apiVersion = "v1beta"
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Client{
		apiKey:     opts.APIKey,
		baseURL:    baseURL,
		apiVersion: apiVersion,
		httpClient: opts.HTTPClient,
		logger:     logger,
	}
}

func (c *Client) Chat(ctx context.Context, history []Message, currentPrompt string, images []ImageInput, opts ChatOptions) (Response, error) {
	model := modelText
	var generationConfig generationConfig
	generationConfig.Temperature = 0.7

	if len(images) > 0 {
		model = modelImage
		if opts.WantImage {
			generationConfig.ResponseModalities = []string{"IMAGE", "TEXT"}
		}
	} else {
		generationConfig.ThinkingConfig = &thinkingConfig{ThinkingBudget: 32768}
	}

	req := generateContentRequest{
		Contents:          buildContents(history, currentPrompt, images, opts),
		SystemInstruction: &content{Role: "user", Parts: []part{{Text: systemInstruction}}},
		GenerationConfig:  generationConfig,
	}

	resp, err := c.generateContent(ctx, model, req)
	if err != nil && generationConfig.ThinkingConfig != nil {
		if isUnknownFieldError(err, "thinkingConfig") {
			generationConfig.ThinkingConfig = nil
			req.GenerationConfig = generationConfig
			return c.generateContent(ctx, model, req)
		}
	}

	if err == nil && opts.WantImage && len(images) > 0 && len(resp.Images) == 0 {
		retryPrompt := strings.TrimSpace(currentPrompt) + "\n\nNatijani faqat tahrirlangan rasm (inlineData) ko'rinishida qaytaring. Matn/JSON/kod yozmang."
		req.Contents = buildContents(history, retryPrompt, images, opts)
		retryResp, retryErr := c.generateContent(ctx, model, req)
		if retryErr == nil && len(retryResp.Images) > 0 {
			return retryResp, nil
		}
	}

	return resp, err
}

func (c *Client) GenerateImage(ctx context.Context, prompt string) ([]string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is empty")
	}

	req := generateContentRequest{
		Contents: []content{
			{Role: "user", Parts: []part{{Text: fmt.Sprintf("Generate a high quality image: %s", prompt)}}},
		},
		SystemInstruction: &content{Role: "user", Parts: []part{{Text: systemInstruction}}},
		GenerationConfig: generationConfig{
			ResponseModalities: []string{"IMAGE"},
			ImageConfig:        &imageConfig{AspectRatio: "1:1"},
		},
	}

	resp, err := c.generateContent(ctx, modelImage, req)
	if err != nil && req.GenerationConfig.ImageConfig != nil {
		if isUnknownFieldError(err, "imageConfig") {
			req.GenerationConfig.ImageConfig = nil
			resp, err = c.generateContent(ctx, modelImage, req)
		}
	}
	if err != nil {
		return nil, err
	}
	return resp.Images, nil
}

func buildContents(history []Message, currentPrompt string, images []ImageInput, opts ChatOptions) []content {
	var contents []content

	for _, msg := range history {
		parts := []part{{Text: msg.Text}}
		for _, imageURL := range msg.ImageURLs {
			if inline, ok := dataURLToInlineData(imageURL, "image/png"); ok {
				parts = append(parts, part{InlineData: &inline})
			}
		}

		role := msg.Role
		if role == "" {
			role = "user"
		}
		contents = append(contents, content{
			Role:  role,
			Parts: parts,
		})
	}

	promptText := strings.TrimSpace(currentPrompt)
	if opts.WantImage && len(images) > 0 {
		promptText += "\n\nQoidalar: natijani rasm sifatida qaytaring (inlineData). JSON/kod/link yozmang."
	}

	var currentParts []part
	if len(images) <= 1 {
		currentParts = []part{{Text: promptText}}
		for _, img := range images {
			currentParts = append(currentParts, part{
				InlineData: &blob{
					Data:     stripDataURLPrefix(img.DataBase64),
					MimeType: img.MimeType,
				},
			})
		}
	} else {
		currentParts = []part{{
			Text: promptText + "\n\nRasm tartibi:\n1) reference/style\n2) target/edit\nQolganlari: qo'shimcha reference.",
		}}

		for i, img := range images {
			label := fmt.Sprintf("Rasm #%d:", i+1)
			if i == 0 {
				label = "Rasm #1 (reference/style):"
			} else if i == 1 {
				label = "Rasm #2 (target/edit):"
			}

			currentParts = append(currentParts,
				part{Text: label},
				part{InlineData: &blob{
					Data:     stripDataURLPrefix(img.DataBase64),
					MimeType: img.MimeType,
				}},
			)
		}
	}

	contents = append(contents, content{
		Role:  "user",
		Parts: currentParts,
	})

	return contents
}

func (c *Client) generateContent(ctx context.Context, model string, payload generateContentRequest) (Response, error) {
	if c.httpClient == nil {
		return Response{}, errors.New("http client is nil")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s/models/%s:generateContent", c.baseURL, c.apiVersion, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.apiKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("request: %w", err)
	}
	defer httpResp.Body.Close()

	rawBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		return Response{}, fmt.Errorf("gemini API %s: %s", httpResp.Status, strings.TrimSpace(string(rawBody)))
	}

	var decoded generateContentResponse
	if err := json.Unmarshal(rawBody, &decoded); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}

	text, images := extractParts(decoded)
	if strings.TrimSpace(text) == "" && len(images) > 0 {
		text = "Rasm tayyor!"
	}
	if strings.TrimSpace(text) == "" && len(images) == 0 {
		text = "Javob olib bo'lmadi."
	}

	return Response{
		Text:   text,
		Images: images,
	}, nil
}

func extractParts(resp generateContentResponse) (string, []string) {
	if len(resp.Candidates) == 0 {
		return "", nil
	}

	var textBuilder strings.Builder
	var images []string

	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" {
			textBuilder.WriteString(p.Text)
		}
		if p.InlineData != nil && p.InlineData.Data != "" && p.InlineData.MimeType != "" {
			images = append(images, fmt.Sprintf("data:%s;base64,%s", p.InlineData.MimeType, p.InlineData.Data))
		}
	}

	return textBuilder.String(), images
}

type generateContentRequest struct {
	Contents          []content        `json:"contents"`
	SystemInstruction *content         `json:"systemInstruction,omitempty"`
	GenerationConfig  generationConfig `json:"generationConfig,omitempty"`
}

type generationConfig struct {
	Temperature        float64         `json:"temperature,omitempty"`
	ResponseModalities []string        `json:"responseModalities,omitempty"`
	ThinkingConfig     *thinkingConfig `json:"thinkingConfig,omitempty"`
	ImageConfig        *imageConfig    `json:"imageConfig,omitempty"`
}

type thinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget,omitempty"`
}

type imageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text       string `json:"text,omitempty"`
	InlineData *blob  `json:"inlineData,omitempty"`
}

type blob struct {
	Data     string `json:"data"`
	MimeType string `json:"mimeType"`
}

type generateContentResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content content `json:"content"`
}

var dataURLRegex = regexp.MustCompile(`^data:([^;]+);base64,`)

func dataURLToInlineData(dataURL string, fallbackMime string) (blob, bool) {
	dataURL = strings.TrimSpace(dataURL)
	if dataURL == "" {
		return blob{}, false
	}

	mime := fallbackMime
	if matches := dataURLRegex.FindStringSubmatch(dataURL); len(matches) == 2 {
		mime = matches[1]
	}

	data := stripDataURLPrefix(dataURL)
	if data == "" {
		return blob{}, false
	}

	return blob{
		Data:     data,
		MimeType: mime,
	}, true
}

func stripDataURLPrefix(value string) string {
	if idx := strings.IndexByte(value, ','); idx >= 0 {
		return value[idx+1:]
	}
	return value
}

func isUnknownFieldError(err error, field string) bool {
	message := err.Error()
	return strings.Contains(message, "Unknown name") && strings.Contains(message, field)
}
