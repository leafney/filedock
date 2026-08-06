package dto

type CreateUploadBatchRequest struct {
	IdempotencyKey string               `json:"idempotencyKey"`
	Scope          string               `json:"scope"`
	RecipientIDs   []string             `json:"recipientIds"`
	Files          []UploadFileManifest `json:"files"`
}

type UploadFileManifest struct {
	OriginalName string `json:"originalName"`
	DeclaredSize int64  `json:"declaredSize"`
	DeclaredMIME string `json:"declaredMime"`
}

type UploadBatchDTO struct {
	BatchID           string          `json:"batchId"`
	Scope             string          `json:"scope"`
	Status            string          `json:"status"`
	DeclaredTotalSize int64           `json:"declaredTotalSize"`
	Files             []UploadFileDTO `json:"files"`
}

type UploadFileDTO struct {
	FileID       string `json:"fileId"`
	UploadID     string `json:"uploadId"`
	DisplayName  string `json:"displayName"`
	PrivateCode  string `json:"privateCode,omitempty"`
	DeclaredSize int64  `json:"declaredSize"`
	Status       string `json:"status"`
	ChunkSize    int64  `json:"chunkSize"`
	TotalParts   int    `json:"totalParts"`
	ExpiresAt    int64  `json:"expiresAt"`
	UploadURL    string `json:"uploadUrl"`
}

type UploadPartDTO struct {
	PartNumber  int    `json:"partNumber"`
	StartOffset int64  `json:"startOffset"`
	EndOffset   int64  `json:"endOffset"`
	Length      int64  `json:"length"`
	SHA256      string `json:"sha256"`
}

type UploadSessionDTO struct {
	UploadID      string          `json:"uploadId"`
	FileID        string          `json:"fileId"`
	Status        string          `json:"status"`
	DeclaredSize  int64           `json:"declaredSize"`
	ChunkSize     int64           `json:"chunkSize"`
	TotalParts    int             `json:"totalParts"`
	ReceivedBytes int64           `json:"receivedBytes"`
	ExpiresAt     int64           `json:"expiresAt"`
	Parts         []UploadPartDTO `json:"parts"`
}

type ReusePrivateFilesRequest struct {
	FileIDs      []string `json:"fileIds"`
	RecipientIDs []string `json:"recipientIds"`
}

type TrashFileRequest struct {
	Reason string `json:"reason"`
}

type TrashFileDTO struct {
	Status    string `json:"status"`
	DeletedAt int64  `json:"deletedAt"`
}

type RejectFileRestoreRequest struct {
	Reason string `json:"reason"`
}
