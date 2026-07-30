package dto

type CreateRoomRequest struct {
	JoinMode        string `json:"joinMode"`
	PIN             string `json:"pin"`
	PINConfirmation string `json:"pinConfirmation"`
}

type JoinRoomRequest struct {
	Confirmed bool   `json:"confirmed"`
	PIN       string `json:"pin"`
}

type CreateJoinRequest struct {
	Confirmed bool `json:"confirmed"`
}

type RoomSummaryDTO struct {
	RoomID    string `json:"roomId"`
	RoomCode  string `json:"roomCode"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"expiresAt"`
}

type RoomMemberDTO struct {
	UserID       string `json:"userId"`
	DisplayName  string `json:"displayName"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	JoinedAt     int64  `json:"joinedAt"`
	OnlineStatus string `json:"onlineStatus"`
}

type RoomSnapshotDTO struct {
	RoomID              string          `json:"roomId"`
	RoomCode            string          `json:"roomCode"`
	Title               string          `json:"title"`
	Status              string          `json:"status"`
	JoinMode            string          `json:"joinMode"`
	Role                string          `json:"role"`
	ExpiresAt           int64           `json:"expiresAt"`
	CanExtend           bool            `json:"canExtend"`
	DestroyAt           *int64          `json:"destroyAt,omitempty"`
	Members             []RoomMemberDTO `json:"members"`
	PendingRequestCount int             `json:"pendingRequestCount,omitempty"`
}

type RoomJoinInfoDTO struct {
	RoomCode       string `json:"roomCode"`
	Title          string `json:"title"`
	JoinMode       string `json:"joinMode"`
	AlreadyMember  bool   `json:"alreadyMember"`
	PendingRequest bool   `json:"pendingRequest"`
}

type ExtendRoomDTO struct {
	ExpiresAt int64 `json:"expiresAt"`
}

type JoinRequestDTO struct {
	RequestID   string `json:"requestId"`
	RoomCode    string `json:"roomCode"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"createdAt"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type JoinRequestListDTO struct {
	Items        []JoinRequestDTO `json:"items"`
	PendingCount int              `json:"pendingCount"`
}

type QRCodeDTO struct {
	SVG string `json:"svg"`
}
