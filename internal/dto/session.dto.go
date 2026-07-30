package dto

type CreateSessionRequest struct {
	DisplayName string `json:"displayName" validate:"required"`
	DeviceHint  string `json:"deviceHint"`
}

type UpdateSessionRequest struct {
	DisplayName string `json:"displayName" validate:"required"`
}

type SessionDTO struct {
	UserID           string `json:"userId"`
	DisplayName      string `json:"displayName"`
	SessionExpiresAt int64  `json:"sessionExpiresAt"`
}

type RandomNicknameDTO struct {
	DisplayName string `json:"displayName"`
}
