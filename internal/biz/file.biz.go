package biz

import (
	"context"
	"fmt"
	"io"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/model"
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
		chunkSize, totalParts := service.UploadChunkPlan(file.DeclaredSize)
		response.Files = append(response.Files, dto.UploadFileDTO{FileID: file.ID, UploadID: file.ID, DisplayName: file.OriginalName, PrivateCode: file.PrivateCode, DeclaredSize: file.DeclaredSize, Status: file.Status, ChunkSize: chunkSize, TotalParts: totalParts, UploadURL: fmt.Sprintf("/api/v1/rooms/%s/files/%s/content", roomCode, file.ID)})
	}
	return response, nil
}

func (b *FileBiz) UploadStatus(userID, roomCode, fileID string) (dto.UploadSessionDTO, error) {
	result, err := b.files.UploadStatus(userID, roomCode, fileID)
	if err != nil {
		return dto.UploadSessionDTO{}, err
	}
	return uploadSessionDTO(result), nil
}

func (b *FileBiz) UploadContent(ctx context.Context, userID, roomCode, fileID string, contentLength int64, source io.Reader) error {
	return b.files.UploadContent(ctx, userID, roomCode, fileID, contentLength, source)
}

func (b *FileBiz) UploadPart(ctx context.Context, userID, roomCode, fileID string, partNumber int, startOffset, endOffset, totalSize, contentLength int64, expectedSHA string, source io.Reader) (dto.UploadSessionDTO, error) {
	result, err := b.files.UploadPart(ctx, userID, roomCode, fileID, partNumber, startOffset, endOffset, totalSize, contentLength, expectedSHA, source)
	if err != nil {
		return dto.UploadSessionDTO{}, err
	}
	return uploadSessionDTO(result), nil
}

func uploadSessionDTO(value service.UploadSessionResult) dto.UploadSessionDTO {
	parts := make([]dto.UploadPartDTO, 0, len(value.Parts))
	for _, part := range value.Parts {
		parts = append(parts, dto.UploadPartDTO{PartNumber: part.PartNumber, StartOffset: part.StartOffset, EndOffset: part.EndOffset, Length: part.Length, SHA256: part.SHA256})
	}
	return dto.UploadSessionDTO{UploadID: value.UploadID, FileID: value.FileID, Status: value.Status, DeclaredSize: value.DeclaredSize, ChunkSize: value.ChunkSize, TotalParts: value.TotalParts, ReceivedBytes: value.ReceivedBytes, ExpiresAt: value.ExpiresAt, Parts: parts}
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

func (b *FileBiz) InspectDownload(userID, roomCode, taskID string) (service.DownloadDescriptor, error) {
	return b.files.InspectDownload(userID, roomCode, taskID)
}

func (b *FileBiz) BeginDownloadRange(userID, roomCode, taskID string, requested *service.ByteRange) (*service.DownloadStream, error) {
	return b.files.BeginDownloadRange(userID, roomCode, taskID, requested)
}

func (b *FileBiz) TrashList(userID, roomCode string, query service.FileTrashListQuery) (service.FileTrashPage, error) {
	return b.files.ListTrash(userID, roomCode, query)
}

func (b *FileBiz) Trash(userID, roomCode, fileID string, request dto.TrashFileRequest) (dto.TrashFileDTO, error) {
	cycle, err := b.files.TrashFile(userID, roomCode, fileID, request.Reason)
	if err != nil {
		return dto.TrashFileDTO{}, err
	}
	return dto.TrashFileDTO{Status: model.FileStatusTrashed, DeletedAt: cycle.DeletedAt}, nil
}

func (b *FileBiz) Restore(userID, roomCode, fileID string) (service.FileRestoreActionResult, error) {
	return b.files.RestoreFile(userID, roomCode, fileID)
}

func (b *FileBiz) ApproveRestore(userID, roomCode, requestID string) error {
	return b.files.ApproveFileRestore(userID, roomCode, requestID)
}

func (b *FileBiz) RejectRestore(userID, roomCode, requestID string, request dto.RejectFileRestoreRequest) error {
	return b.files.RejectFileRestore(userID, roomCode, requestID, request.Reason)
}

func (b *FileBiz) Purge(userID, roomCode, fileID string) error {
	return b.files.PurgeFile(userID, roomCode, fileID)
}
