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
	DisplayName  string `json:"displayName"`
	PrivateCode  string `json:"privateCode,omitempty"`
	DeclaredSize int64  `json:"declaredSize"`
	Status       string `json:"status"`
	UploadURL    string `json:"uploadUrl"`
}
