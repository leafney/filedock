package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	DownloadTaskTTL     = 10 * time.Minute
	DownloadGlobalLimit = 50
	DownloadRoomLimit   = 10
	DownloadUserLimit   = 2
)

type DownloadTaskResult struct {
	TaskID      string `json:"taskId"`
	FileID      string `json:"fileId"`
	FileName    string `json:"fileName"`
	Size        int64  `json:"size"`
	ExpiresAt   int64  `json:"expiresAt"`
	DownloadURL string `json:"downloadUrl"`
}

type DownloadStream struct {
	svc         *FileSvc
	task        model.DownloadTask
	fileRecord  model.RoomFile
	file        *os.File
	fileName    string
	mime        string
	finish      func()
	taskContext context.Context
	once        sync.Once
}

type downloadRegistry struct {
	mu     sync.Mutex
	active map[string]downloadActive
	rooms  map[string]int
	users  map[string]int
}

type downloadActive struct {
	taskID string
	roomID string
	fileID string
	userID string
	cancel context.CancelFunc
}

func (s *FileSvc) CreateDownloadTask(userID, roomCode, fileID string) (DownloadTaskResult, error) {
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return DownloadTaskResult{}, err
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ? AND status = ?", fileID, room.ID, model.FileStatusAvailable).First(&file).Error; err != nil {
		return DownloadTaskResult{}, fileNotFound(err)
	}
	allowed, err := s.canDownload(userID, file)
	if err != nil {
		return DownloadTaskResult{}, err
	}
	if !allowed {
		return DownloadTaskResult{}, errx.New(errc.ErrFileNotFound, nil)
	}
	taskID, err := ulidx.New()
	if err != nil {
		return DownloadTaskResult{}, err
	}
	now := s.now()
	task := model.DownloadTask{ID: taskID, RoomID: room.ID, FileID: file.ID, UserID: userID, ExpectedSize: file.ActualSize, Status: model.DownloadTaskPending, CreatedAt: now.Unix(), ExpiresAt: now.Add(DownloadTaskTTL).Unix()}
	if err := s.db.Create(&task).Error; err != nil {
		return DownloadTaskResult{}, err
	}
	return DownloadTaskResult{TaskID: task.ID, FileID: file.ID, FileName: file.OriginalName, Size: file.ActualSize, ExpiresAt: task.ExpiresAt, DownloadURL: fmt.Sprintf("/api/v1/rooms/%s/downloads/%s", roomCode, task.ID)}, nil
}

func (s *FileSvc) AcceptAndCreateDownload(userID, roomCode, fileID string) (DownloadTaskResult, error) {
	if err := s.AcceptFile(userID, roomCode, fileID); err != nil {
		return DownloadTaskResult{}, err
	}
	return s.CreateDownloadTask(userID, roomCode, fileID)
}

func (s *FileSvc) BeginDownload(userID, roomCode, taskID string) (*DownloadStream, error) {
	if s == nil || s.db == nil || s.storage == nil || s.downloads == nil {
		return nil, fmt.Errorf("file download service is unavailable")
	}
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return nil, err
	}
	var task model.DownloadTask
	if err := s.db.Where("id = ? AND room_id = ? AND user_id = ?", taskID, room.ID, userID).First(&task).Error; err != nil {
		return nil, fileNotFound(err)
	}
	if task.Status == model.DownloadTaskCancelled {
		return nil, errx.New(errc.ErrDownloadCancelled, nil)
	}
	if task.Status != model.DownloadTaskPending || task.ExpiresAt <= s.now().Unix() {
		if task.Status == model.DownloadTaskPending && task.ExpiresAt <= s.now().Unix() {
			_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskPending).Update("status", model.DownloadTaskExpired).Error
		}
		return nil, errx.New(errc.ErrDownloadExpired, nil)
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ? AND status = ?", task.FileID, room.ID, model.FileStatusAvailable).First(&file).Error; err != nil {
		return nil, fileNotFound(err)
	}
	allowed, err := s.canDownload(userID, file)
	if err != nil || !allowed {
		if err != nil {
			return nil, err
		}
		return nil, errx.New(errc.ErrFileNotFound, nil)
	}
	taskContext, finish, err := s.downloads.begin(task.ID, room.ID, file.ID, userID)
	if err != nil {
		return nil, err
	}
	now := s.now().Unix()
	result := s.db.Model(&model.DownloadTask{}).Where("id = ? AND user_id = ? AND status = ? AND expires_at > ?", task.ID, userID, model.DownloadTaskPending, now).Updates(map[string]interface{}{"status": model.DownloadTaskStreaming, "started_at": now})
	if result.Error != nil || result.RowsAffected != 1 {
		finish()
		if result.Error != nil {
			return nil, result.Error
		}
		return nil, errx.New(errc.ErrDownloadExpired, nil)
	}
	stored, info, err := s.storage.Open(file.RoomID, file.StorageName)
	if err != nil {
		finish()
		_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
		return nil, errx.Wrap(errc.ErrFileStorage, err, nil)
	}
	if info.Size() != task.ExpectedSize || info.Size() != file.ActualSize {
		_ = stored.Close()
		finish()
		_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
		return nil, errx.New(errc.ErrFileStorage, nil)
	}
	if err := createFileEvent(s.db, file.RoomID, file.ID, file.BatchID, userID, FileEventDownloadStarted, now); err != nil {
		_ = stored.Close()
		finish()
		_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
		return nil, err
	}
	s.publishFileProjection(file.ID, "file.download_started", map[string]interface{}{"downloaderUserId": userID})
	mimeType := file.DetectedMIME
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	task.Status = model.DownloadTaskStreaming
	task.StartedAt = &now
	return &DownloadStream{svc: s, task: task, fileRecord: file, file: stored, fileName: file.OriginalName, mime: mimeType, finish: finish, taskContext: taskContext}, nil
}

func (stream *DownloadStream) FileName() string { return stream.fileName }
func (stream *DownloadStream) MIME() string     { return stream.mime }
func (stream *DownloadStream) Size() int64      { return stream.task.ExpectedSize }

func (stream *DownloadStream) WriteTo(ctx context.Context, target io.Writer) error {
	if stream == nil || stream.svc == nil || stream.file == nil || target == nil {
		return fmt.Errorf("download stream is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	defer stream.close()
	buffer := make([]byte, fileCopyBufferSize)
	var transferred int64
	lastProgress := 0
	lastPublished := time.Time{}
	for {
		if err := stream.taskContext.Err(); err != nil {
			stream.fail(transferred)
			return err
		}
		if err := ctx.Err(); err != nil {
			stream.fail(transferred)
			return err
		}
		read, readErr := stream.file.Read(buffer)
		if read > 0 {
			written, writeErr := target.Write(buffer[:read])
			transferred += int64(written)
			if writeErr != nil || written != read {
				stream.fail(transferred)
				if writeErr != nil {
					return writeErr
				}
				return io.ErrShortWrite
			}
			progress := int(transferred * 100 / stream.task.ExpectedSize)
			now := stream.svc.now()
			if progress > lastProgress && (progress == 100 || now.Sub(lastPublished) >= 250*time.Millisecond) {
				lastProgress = progress
				lastPublished = now
				_ = stream.svc.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", stream.task.ID, model.DownloadTaskStreaming).Update("transferred_size", transferred).Error
				if stream.svc.hub != nil {
					stream.svc.hub.PublishUser(stream.task.UserID, "file.download_progress", map[string]interface{}{"taskId": stream.task.ID, "fileId": stream.fileRecord.ID, "progress": progress, "transferred": transferred, "total": stream.task.ExpectedSize})
				}
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			stream.fail(transferred)
			return readErr
		}
	}
	if flusher, ok := target.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			stream.fail(transferred)
			return err
		}
	}
	if transferred != stream.task.ExpectedSize {
		stream.fail(transferred)
		return io.ErrUnexpectedEOF
	}
	if err := stream.complete(transferred); err != nil {
		stream.fail(transferred)
		return err
	}
	stream.svc.publishFileProjection(stream.fileRecord.ID, "file.download_completed", map[string]interface{}{"downloaderUserId": stream.task.UserID})
	return nil
}

func (stream *DownloadStream) complete(transferred int64) error {
	now := stream.svc.now().Unix()
	return stream.svc.db.Transaction(func(tx *gorm.DB) error {
		var current model.RoomFile
		if err := tx.Where("id = ? AND status = ? AND trash_version = ?", stream.fileRecord.ID, model.FileStatusAvailable, stream.fileRecord.TrashVersion).First(&current).Error; err != nil {
			return fileNotFound(err)
		}
		result := tx.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", stream.task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskCompleted, "transferred_size": transferred, "completed_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errx.New(errc.ErrDownloadExpired, nil)
		}
		var recipient model.FileRecipient
		firstDownload := false
		if stream.fileRecord.Scope == model.FileScopeDirect && stream.fileRecord.UploaderUserID != stream.task.UserID {
			if err := tx.Where("file_id = ? AND recipient_user_id = ? AND status IN ?", stream.fileRecord.ID, stream.task.UserID, []string{model.RecipientAccepted, model.RecipientDownloaded}).First(&recipient).Error; err != nil {
				return fileNotFound(err)
			}
			firstDownload = recipient.FirstDownloadedAt == nil
			update := tx.Model(&model.FileRecipient{}).Where("id = ? AND delivery_version = ? AND status IN ?", recipient.ID, recipient.DeliveryVersion, []string{model.RecipientAccepted, model.RecipientDownloaded}).Updates(map[string]interface{}{"status": model.RecipientDownloaded, "first_downloaded_at": gorm.Expr("COALESCE(first_downloaded_at, ?)", now), "last_downloaded_at": now, "download_count": gorm.Expr("download_count + 1")})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return errx.New(errc.ErrFileRecipientState, nil)
			}
		}
		event, err := createFileEventRecord(tx, stream.fileRecord.RoomID, stream.fileRecord.ID, stream.fileRecord.BatchID, stream.task.UserID, FileEventDownloaded, now)
		if err != nil {
			return err
		}
		if firstDownload {
			return stream.svc.recordFileNotification(tx, NotificationTypeFileDownloaded, stream.fileRecord.UploaderUserID, stream.task.UserID, stream.fileRecord, recipient, event)
		}
		return nil
	})
}

func (stream *DownloadStream) fail(transferred int64) {
	now := stream.svc.now().Unix()
	_ = stream.svc.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", stream.task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "transferred_size": transferred, "failed_at": now}).Error
}

func (stream *DownloadStream) close() {
	stream.once.Do(func() {
		_ = stream.file.Close()
		if stream.finish != nil {
			stream.finish()
		}
	})
}

func (s *FileSvc) canDownload(userID string, file model.RoomFile) (bool, error) {
	if file.Scope == model.FileScopeShared || file.UploaderUserID == userID {
		return true, nil
	}
	var recipient model.FileRecipient
	if err := s.db.Where("file_id = ? AND recipient_user_id = ?", file.ID, userID).First(&recipient).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return recipient.Status == model.RecipientAccepted || recipient.Status == model.RecipientDownloaded, nil
}

func (r *downloadRegistry) begin(taskID, roomID, fileID, userID string) (context.Context, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.active[taskID]; exists || len(r.active) >= DownloadGlobalLimit || r.rooms[roomID] >= DownloadRoomLimit || r.users[userID] >= DownloadUserLimit {
		return nil, nil, errx.New(errc.ErrDownloadLimited, nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.active[taskID] = downloadActive{taskID: taskID, roomID: roomID, fileID: fileID, userID: userID, cancel: cancel}
	r.rooms[roomID]++
	r.users[userID]++
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.active, taskID)
			r.rooms[roomID]--
			r.users[userID]--
			r.mu.Unlock()
			cancel()
		})
	}, nil
}

func (r *downloadRegistry) cancelFile(fileID string) []downloadActive {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	matched := make([]downloadActive, 0)
	for _, active := range r.active {
		if active.fileID == fileID {
			matched = append(matched, active)
		}
	}
	r.mu.Unlock()
	for _, active := range matched {
		active.cancel()
	}
	return matched
}

func (r *downloadRegistry) cancelMatching(roomID, userID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	cancels := make([]context.CancelFunc, 0)
	for _, active := range r.active {
		if active.roomID == roomID && (userID == "" || active.userID == userID) {
			cancels = append(cancels, active.cancel)
		}
	}
	r.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
