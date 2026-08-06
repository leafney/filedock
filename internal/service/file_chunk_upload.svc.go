package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

type UploadPartView struct {
	PartNumber  int    `json:"partNumber"`
	StartOffset int64  `json:"startOffset"`
	EndOffset   int64  `json:"endOffset"`
	Length      int64  `json:"length"`
	SHA256      string `json:"sha256"`
}

type UploadSessionResult struct {
	UploadID      string           `json:"uploadId"`
	FileID        string           `json:"fileId"`
	Status        string           `json:"status"`
	DeclaredSize  int64            `json:"declaredSize"`
	ChunkSize     int64            `json:"chunkSize"`
	TotalParts    int              `json:"totalParts"`
	ReceivedBytes int64            `json:"receivedBytes"`
	ExpiresAt     int64            `json:"expiresAt"`
	Parts         []UploadPartView `json:"parts"`
}

func uploadChunkPlan(size int64) (int64, int) {
	if size <= UploadSmallFileLimit {
		return size, 1
	}
	parts := int((size + UploadChunkSize - 1) / UploadChunkSize)
	return UploadChunkSize, parts
}

func UploadChunkPlan(size int64) (int64, int) {
	return uploadChunkPlan(size)
}

func uploadPartBounds(size, chunkSize int64, totalParts, partNumber int) (int64, int64, bool) {
	if size <= 0 || chunkSize <= 0 || totalParts <= 0 || partNumber < 0 || partNumber >= totalParts {
		return 0, 0, false
	}
	start := int64(partNumber) * chunkSize
	if start >= size {
		return 0, 0, false
	}
	end := start + chunkSize - 1
	if end >= size {
		end = size - 1
	}
	return start, end, true
}

func (s *FileSvc) UploadStatus(userID, roomCode, fileID string) (UploadSessionResult, error) {
	if s == nil || s.db == nil {
		return UploadSessionResult{}, fmt.Errorf("file upload service is unavailable")
	}
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return UploadSessionResult{}, err
	}
	var session model.UploadSession
	if err := s.db.Where("file_id = ? AND room_id = ? AND uploader_user_id = ?", fileID, room.ID, userID).First(&session).Error; err != nil {
		return UploadSessionResult{}, uploadSessionNotFound(err)
	}
	return s.uploadSessionResult(session), nil
}

func (s *FileSvc) ExpireUploadSessions(now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("file upload service is unavailable")
	}
	if now.IsZero() {
		now = s.now()
	}
	var sessions []model.UploadSession
	if err := s.db.Where("status = ? AND expires_at <= ?", model.UploadSessionActive, now.Unix()).Find(&sessions).Error; err != nil {
		return err
	}
	for _, session := range sessions {
		var file model.RoomFile
		if err := s.db.Where("id = ?", session.FileID).First(&file).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return err
		}
		if file.Status == model.FileStatusReserved || file.Status == model.FileStatusUploading {
			if err := s.ReleaseUpload(file.UploaderUserID, file.ID, model.FileStatusFailed, errc.ErrUploadExpired); err != nil && errx.Code(err) != errc.ErrFileState {
				return err
			}
		}
		if err := s.db.Where("file_id = ?", file.ID).Delete(&model.UploadPart{}).Error; err != nil {
			return err
		}
		if err := s.db.Where("file_id = ?", file.ID).Delete(&model.UploadSession{}).Error; err != nil {
			return err
		}
		if s.storage != nil {
			if err := s.storage.DeleteTemporaryFile(file.RoomID, file.StorageName); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *FileSvc) UploadPart(ctx context.Context, userID, roomCode, fileID string, partNumber int, startOffset, endOffset, totalSize, contentLength int64, expectedSHA string, source io.Reader) (UploadSessionResult, error) {
	if s == nil || s.db == nil || s.storage == nil || source == nil {
		return UploadSessionResult{}, fmt.Errorf("file upload service is unavailable")
	}
	if partNumber < 0 || startOffset < 0 || endOffset < startOffset || contentLength <= 0 || totalSize <= 0 || endOffset-startOffset+1 != contentLength || strings.TrimSpace(expectedSHA) == "" {
		return UploadSessionResult{}, errx.New(errc.ErrUploadChunkInvalid, nil)
	}
	body, err := io.ReadAll(io.LimitReader(source, contentLength+1))
	if err != nil {
		return UploadSessionResult{}, errx.Wrap(errc.ErrUploadSize, err, nil)
	}
	if int64(len(body)) != contentLength {
		return UploadSessionResult{}, errx.New(errc.ErrUploadSize, nil)
	}
	actualSHA := chunkSHA256(body)
	if !strings.EqualFold(strings.TrimSpace(expectedSHA), actualSHA) {
		return UploadSessionResult{}, errx.New(errc.ErrUploadChunkHash, nil)
	}

	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return UploadSessionResult{}, err
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ? AND uploader_user_id = ?", fileID, room.ID, userID).First(&file).Error; err != nil {
		return UploadSessionResult{}, fileNotFound(err)
	}
	var session model.UploadSession
	if err := s.db.Where("file_id = ? AND room_id = ? AND uploader_user_id = ?", fileID, room.ID, userID).First(&session).Error; err != nil {
		return UploadSessionResult{}, uploadSessionNotFound(err)
	}
	if session.ExpiresAt <= s.now().Unix() {
		return UploadSessionResult{}, errx.New(errc.ErrUploadExpired, nil)
	}
	if session.Status == model.UploadSessionCompleted || file.Status == model.FileStatusAvailable {
		var completedPart model.UploadPart
		if err := s.db.Where("file_id = ? AND part_number = ?", fileID, partNumber).First(&completedPart).Error; err == nil && completedPart.StartOffset == startOffset && completedPart.EndOffset == endOffset && completedPart.Length == contentLength && strings.EqualFold(completedPart.SHA256, actualSHA) {
			return s.UploadStatus(userID, roomCode, fileID)
		}
		return UploadSessionResult{}, errx.New(errc.ErrFileState, nil)
	}
	if session.Status != model.UploadSessionActive || (file.Status != model.FileStatusReserved && file.Status != model.FileStatusUploading) {
		return UploadSessionResult{}, uploadSessionStateError(session.Status)
	}
	wantStart, wantEnd, ok := uploadPartBounds(session.DeclaredSize, session.ChunkSize, session.TotalParts, partNumber)
	if !ok || totalSize != session.DeclaredSize || startOffset != wantStart || endOffset != wantEnd || contentLength != wantEnd-wantStart+1 {
		return UploadSessionResult{}, errx.New(errc.ErrUploadChunkInvalid, nil)
	}
	var existing model.UploadPart
	if err := s.db.Where("file_id = ? AND part_number = ?", fileID, partNumber).First(&existing).Error; err == nil {
		if existing.StartOffset != startOffset || existing.EndOffset != endOffset || existing.Length != contentLength || !strings.EqualFold(existing.SHA256, actualSHA) {
			return UploadSessionResult{}, errx.New(errc.ErrUploadChunkConflict, nil)
		}
		return s.UploadStatus(userID, roomCode, fileID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return UploadSessionResult{}, err
	}

	partKey := fmt.Sprintf("%s:%d", fileID, partNumber)
	partContext, finish, err := s.uploads.beginPart(ctx, partKey, file.ID, file.RoomID, userID)
	if err != nil {
		return UploadSessionResult{}, err
	}
	defer finish()
	if file.Status == model.FileStatusReserved {
		if err := s.MarkUploading(userID, file.ID); err != nil && errx.Code(err) != errc.ErrFileState {
			return UploadSessionResult{}, err
		}
	}
	if err := s.storage.WritePart(partContext, file.RoomID, file.StorageName, startOffset, contentLength, session.DeclaredSize, bytes.NewReader(body)); err != nil {
		return UploadSessionResult{}, errx.Wrap(errc.ErrFileStorage, err, nil)
	}
	now := s.now().Unix()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var current model.UploadSession
		if err := tx.Where("file_id = ? AND status = ?", file.ID, model.UploadSessionActive).First(&current).Error; err != nil {
			return uploadSessionNotFound(err)
		}
		var duplicate model.UploadPart
		if err := tx.Where("file_id = ? AND part_number = ?", file.ID, partNumber).First(&duplicate).Error; err == nil {
			if duplicate.SHA256 != actualSHA || duplicate.StartOffset != startOffset || duplicate.EndOffset != endOffset || duplicate.Length != contentLength {
				return errx.New(errc.ErrUploadChunkConflict, nil)
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		partID, err := ulidx.New()
		if err != nil {
			return err
		}
		if err := tx.Create(&model.UploadPart{ID: partID, FileID: file.ID, PartNumber: partNumber, StartOffset: startOffset, EndOffset: endOffset, Length: contentLength, SHA256: actualSHA, CompletedAt: now}).Error; err != nil {
			return err
		}
		var received int64
		if err := tx.Model(&model.UploadPart{}).Where("file_id = ?", file.ID).Select("COALESCE(SUM(length), 0)").Scan(&received).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.UploadSession{}).Where("file_id = ? AND status = ?", file.ID, model.UploadSessionActive).Updates(map[string]interface{}{"received_bytes": received, "updated_at": now}).Error; err != nil {
			return err
		}
		progress := int(received * 100 / current.DeclaredSize)
		return tx.Model(&model.RoomFile{}).Where("id = ? AND status = ?", file.ID, model.FileStatusUploading).Update("progress", progress).Error
	}); err != nil {
		return UploadSessionResult{}, err
	}
	if current, statusErr := s.UploadStatus(userID, roomCode, file.ID); statusErr == nil && current.Status == model.UploadSessionActive {
		s.publishFileProjection(file.ID, "file.upload_progress", map[string]interface{}{"progress": int(current.ReceivedBytes * 100 / current.DeclaredSize), "receivedBytes": current.ReceivedBytes, "totalBytes": current.DeclaredSize})
	}
	if err := s.finalizeChunkedUpload(userID, roomCode, file.ID); err != nil {
		return UploadSessionResult{}, err
	}
	return s.UploadStatus(userID, roomCode, file.ID)
}

func (s *FileSvc) finalizeChunkedUpload(userID, roomCode, fileID string) error {
	s.finalizeMu.Lock()
	defer s.finalizeMu.Unlock()
	var session model.UploadSession
	if err := s.db.Where("file_id = ? AND uploader_user_id = ?", fileID, userID).First(&session).Error; err != nil {
		return uploadSessionNotFound(err)
	}
	if session.Status == model.UploadSessionCompleted {
		return nil
	}
	if session.Status != model.UploadSessionActive || session.ReceivedBytes != session.DeclaredSize {
		return nil
	}
	var parts int64
	if err := s.db.Model(&model.UploadPart{}).Where("file_id = ?", fileID).Count(&parts).Error; err != nil {
		return err
	}
	if int(parts) != session.TotalParts {
		return nil
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND uploader_user_id = ?", fileID, userID).First(&file).Error; err != nil {
		return fileNotFound(err)
	}
	stored, err := s.storage.FinalizeUpload(file.RoomID, file.StorageName, session.DeclaredSize)

	if err != nil {
		return errx.Wrap(errc.ErrFileStorage, err, nil)
	}
	if err := s.CompleteUpload(userID, file.ID, stored.DetectedMIME, stored.Size); err != nil {
		return err
	}
	s.refreshBatchStatus(file.BatchID)
	s.publishFileProjection(file.ID, "file.available", map[string]interface{}{"progress": 100})
	return nil
}

func (s *FileSvc) uploadSessionResult(session model.UploadSession) UploadSessionResult {
	result := UploadSessionResult{UploadID: session.FileID, FileID: session.FileID, Status: session.Status, DeclaredSize: session.DeclaredSize, ChunkSize: session.ChunkSize, TotalParts: session.TotalParts, ReceivedBytes: session.ReceivedBytes, ExpiresAt: session.ExpiresAt, Parts: []UploadPartView{}}
	var parts []model.UploadPart
	if err := s.db.Where("file_id = ?", session.FileID).Order("part_number ASC").Find(&parts).Error; err != nil {
		return result
	}
	result.Parts = make([]UploadPartView, 0, len(parts))
	for _, part := range parts {
		result.Parts = append(result.Parts, UploadPartView{PartNumber: part.PartNumber, StartOffset: part.StartOffset, EndOffset: part.EndOffset, Length: part.Length, SHA256: part.SHA256})
	}
	return result
}

func uploadSessionNotFound(err error) error {
	if err == nil {
		return errx.New(errc.ErrFileNotFound, nil)
	}
	return fileNotFound(err)
}

func uploadSessionStateError(status string) error {
	switch status {
	case model.UploadSessionCancelled:
		return errx.New(errc.ErrUploadCancelled, nil)
	case model.UploadSessionExpired:
		return errx.New(errc.ErrUploadExpired, nil)
	default:
		return errx.New(errc.ErrFileState, nil)
	}
}

func chunkSHA256(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
