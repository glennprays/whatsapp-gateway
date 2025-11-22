package whatsapp

type Webhook struct {
	Url        string `json:"url" binding:"required,url"`
	HmacSecret string `json:"hmac_secret" binding:"omitempty"`
}
