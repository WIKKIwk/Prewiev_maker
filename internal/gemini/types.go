package gemini

type Message struct {
	Role      string
	Text      string
	ImageURLs []string
}

type ImageInput struct {
	DataBase64 string
	MimeType   string
}

type Response struct {
	Text   string
	Images []string
}
