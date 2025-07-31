package domain

type RegistrationRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required,numeric,min=10,max=18"`
	SecretKey   string `json:"secret_key" binding:"required"`
}

type RegistrationResponse struct {
	Token string `json:"token"`
}
