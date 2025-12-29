package handlers

import "strings"

func wantsImageOutput(prompt string, imageCount int) bool {
	if imageCount >= 2 {
		return true
	}

	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return false
	}

	keywords := []string{
		"qo'y", "qo‘y",
		"o'zgart", "o‘zgart",
		"tahrir",
		"joylashtir",
		"almashtir",
		"ustiga", "ustidan",
		"qilib ber", "qilib bera",
		"propors",
		"tekstura", "texture",
		"edit", "change", "apply", "replace", "put", "remove", "add",
	}

	for _, kw := range keywords {
		if strings.Contains(p, kw) {
			return true
		}
	}

	return false
}

func looksLikeToolCall(text string) bool {
	t := strings.ToLower(text)
	return strings.Contains(t, "generate_image") ||
		strings.Contains(t, "\"text_prompt\"") ||
		strings.Contains(t, "\"image_prompt\"") ||
		strings.Contains(t, "image_strength") ||
		strings.Contains(t, "image_url")
}
