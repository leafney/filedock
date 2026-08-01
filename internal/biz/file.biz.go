package biz

import (
	"context"
	"fmt"
	"io"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
)

type FileBiz struct {
	files *service.FileSvc
}

func NewFileBiz(files *service.FileSvc) (*FileBiz, error) {
	if files == nil {
		return nil, fmt.Errorf("file service is required")
	}
	return &FileBiz{files: files}, nil
}

func (b *FileBiz) CreateUploadBatch(userID, roomCode string, request dto.CreateUploadBatchRequest) (dto.UploadBatchDTO, error) {
	manifests := make([]service.FileManifest, 0, len(request.Files))
	for _, file := range request.Files {
		manifests = append(manifests, service.FileManifest{OriginalName: file.OriginalName, DeclaredSize: file.DeclaredSize, DeclaredMIME: file.DeclaredMIME})
	}
	result, err := b.files.CreateUploadBatch(userID, roomCode, request.IdempotencyKey, request.Scope, manifests, request.RecipientIDs)
	if err != nil {
		return dto.UploadBatchDTO{}, err
	}
	response := dto.UploadBatchDTO{BatchID: result.Batch.ID, Scope: result.Batch.Scope, Status: result.Batch.Status, DeclaredTotalSize: result.Batch.DeclaredTotalSize, Files: make([]dto.UploadFileDTO, 0, len(result.Files))}
	for _, file := range result.Files {
		response.Files = append(response.Files, dto.UploadFileDTO{FileID: file.ID, DisplayName: file.OriginalName, PrivateCode: file.PrivateCode, DeclaredSize: file.DeclaredSize, Status: file.Status, UploadURL: fmt.Sprintf("/api/v1/rooms/%s/files/%s/content", roomCode, file.ID)})
	}
	return response, nil
}

func (b *FileBiz) UploadContent(ctx context.Context, userID, roomCode, fileID string, contentLength int64, source io.Reader) error {
	return b.files.UploadContent(ctx, userID, roomCode, fileID, contentLength, source)
}

func (b *FileBiz) CancelUpload(userID, roomCode, fileID string) error {
	return b.files.CancelUpload(userID, roomCode, fileID)
}

func (b *FileBiz) List(userID, roomCode string, query service.FileListQuery) (service.FileListResult, error) {
	return b.files.ListFiles(userID, roomCode, query)
}

func (b *FileBiz) Events(userID, roomCode, cursor string, limit int) (service.FileEventPage, error) {
	return b.files.ListFileEvents(userID, roomCode, cursor, limit)
}

func (b *FileBiz) Reusable(userID, roomCode string) ([]service.FileProjection, error) {
	return b.files.ReusableFiles(userID, roomCode)
}

func (b *FileBiz) Accept(userID, roomCode, fileID string) error {
	return b.files.AcceptFile(userID, roomCode, fileID)
}

func (b *FileBiz) Decline(userID, roomCode, fileID string) error {
	return b.files.DeclineFile(userID, roomCode, fileID)
}

func (b *FileBiz) Reuse(userID, roomCode string, request dto.ReusePrivateFilesRequest) (service.ReuseResult, error) {
	return b.files.ReusePrivateFiles(userID, roomCode, request.FileIDs, request.RecipientIDs)
}

func (b *FileBiz) PublishShared(userID, roomCode, fileID string) error {
	return b.files.PublishShared(userID, roomCode, fileID)
}

func (b *FileBiz) CreateDownload(userID, roomCode, fileID string) (service.DownloadTaskResult, error) {
	return b.files.CreateDownloadTask(userID, roomCode, fileID)
}

func (b *FileBiz) AcceptAndDownload(userID, roomCode, fileID string) (service.DownloadTaskResult, error) {
	return b.files.AcceptAndCreateDownload(userID, roomCode, fileID)
}

func (b *FileBiz) BeginDownload(userID, roomCode, taskID string) (*service.DownloadStream, error) {
	return b.files.BeginDownload(userID, roomCode, taskID)
}
