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
	DownloadTaskTTL      = 10 * time.Minute
	DownloadReplayWindow = 2 * time.Second
	DownloadGlobalLimit  = 50
	DownloadRoomLimit    = 10
	DownloadUserLimit    = 2
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
	svc           *FileSvc
	task          model.DownloadTask
	fileRecord    model.RoomFile
	file          *os.File
	fileName      string
	mime          string
	finish        func()
	taskContext   context.Context
	rangeSpec     ByteRange
	hasRange      bool
	replay        bool
	allowComplete bool
	once          sync.Once
}

// DownloadDescriptor contains the validated metadata shared by GET and HEAD.
// It deliberately does not open the stored file or consume a pending task.
type DownloadDescriptor struct {
	Task     model.DownloadTask
	File     model.RoomFile
	FileName string
	MIME     string
	Size     int64
}

func downloadReplayOpen(timestamp *int64, now time.Time) bool {
	if timestamp == nil {
		return false
	}
	started := time.Unix(*timestamp, 0)
	return !now.Before(started) && now.Sub(started) <= DownloadReplayWindow
}

type downloadRegistry struct {
	mu     sync.Mutex
	active map[string]downloadActive
	rooms  map[string]int
	users  map[string]int
	next   uint64
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
	return s.BeginDownloadRange(userID, roomCode, taskID, nil)
}

// InspectDownload validates access and returns metadata without consuming the task.
func (s *FileSvc) InspectDownload(userID, roomCode, taskID string) (DownloadDescriptor, error) {
	if s == nil || s.db == nil || s.storage == nil || s.downloads == nil {
		return DownloadDescriptor{}, fmt.Errorf("file download service is unavailable")
	}
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return DownloadDescriptor{}, err
	}
	var task model.DownloadTask
	if err := s.db.Where("id = ? AND room_id = ? AND user_id = ?", taskID, room.ID, userID).First(&task).Error; err != nil {
		return DownloadDescriptor{}, fileNotFound(err)
	}
	if task.Status == model.DownloadTaskCancelled {
		return DownloadDescriptor{}, errx.New(errc.ErrDownloadCancelled, nil)
	}
	now := s.now()
	if task.Status == model.DownloadTaskPending && task.ExpiresAt <= now.Unix() {
		_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskPending).Update("status", model.DownloadTaskExpired).Error
		return DownloadDescriptor{}, errx.New(errc.ErrDownloadExpired, nil)
	}
	if task.Status == model.DownloadTaskStreaming && !downloadReplayOpen(task.StartedAt, now) {
		return DownloadDescriptor{}, errx.New(errc.ErrDownloadExpired, nil)
	}
	if task.Status == model.DownloadTaskCompleted && !downloadReplayOpen(task.CompletedAt, now) {
		return DownloadDescriptor{}, errx.New(errc.ErrDownloadExpired, nil)
	}
	if task.Status != model.DownloadTaskPending && task.Status != model.DownloadTaskStreaming && task.Status != model.DownloadTaskCompleted {
		return DownloadDescriptor{}, errx.New(errc.ErrDownloadExpired, nil)
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ? AND status = ?", task.FileID, room.ID, model.FileStatusAvailable).First(&file).Error; err != nil {
		return DownloadDescriptor{}, fileNotFound(err)
	}
	allowed, err := s.canDownload(userID, file)
	if err != nil || !allowed {
		if err != nil {
			return DownloadDescriptor{}, err
		}
		return DownloadDescriptor{}, errx.New(errc.ErrFileNotFound, nil)
	}
	mimeType := file.DetectedMIME
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return DownloadDescriptor{Task: task, File: file, FileName: file.OriginalName, MIME: mimeType, Size: file.ActualSize}, nil
}

// BeginDownloadRange starts a validated full or single-range transfer.
func (s *FileSvc) BeginDownloadRange(userID, roomCode, taskID string, requested *ByteRange) (*DownloadStream, error) {
	descriptor, err := s.InspectDownload(userID, roomCode, taskID)
	if err != nil {
		return nil, err
	}
	rangeSpec := ByteRange{Start: 0, End: descriptor.Size - 1}
	hasRange := requested != nil
	if hasRange {
		rangeSpec = *requested
	}
	if (hasRange && (descriptor.Size == 0 || rangeSpec.Start < 0 || rangeSpec.End < rangeSpec.Start || rangeSpec.End >= descriptor.Size)) || (!hasRange && descriptor.Size > 0 && rangeSpec.End >= descriptor.Size) {
		return nil, errx.New(errc.ErrDownloadRangeInvalid, nil)
	}
	task := descriptor.Task
	replay := task.Status == model.DownloadTaskStreaming || task.Status == model.DownloadTaskCompleted
	taskContext, finish, err := s.downloads.begin(task.ID, descriptor.File.RoomID, descriptor.File.ID, userID)
	if err != nil {
		return nil, err
	}
	now := s.now().Unix()
	if !replay {
		result := s.db.Model(&model.DownloadTask{}).Where("id = ? AND user_id = ? AND status = ? AND expires_at > ?", task.ID, userID, model.DownloadTaskPending, now).Updates(map[string]interface{}{"status": model.DownloadTaskStreaming, "started_at": now})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				finish()
				return nil, result.Error
			}
			var current model.DownloadTask
			if loadErr := s.db.First(&current, "id = ?", task.ID).Error; loadErr != nil {
				finish()
				return nil, fileNotFound(loadErr)
			}
			currentReplayOpen := (current.Status == model.DownloadTaskStreaming && downloadReplayOpen(current.StartedAt, s.now())) || (current.Status == model.DownloadTaskCompleted && downloadReplayOpen(current.CompletedAt, s.now()))
			if !currentReplayOpen {
				finish()
				return nil, errx.New(errc.ErrDownloadExpired, nil)
			}
			task = current
			replay = true
		} else {
			task.Status = model.DownloadTaskStreaming
			task.StartedAt = &now
		}
	}
	stored, info, err := s.storage.Open(descriptor.File.RoomID, descriptor.File.StorageName)
	if err != nil {
		finish()
		if !replay {
			_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
		}
		return nil, errx.Wrap(errc.ErrFileStorage, err, nil)
	}
	if info.Size() != task.ExpectedSize || info.Size() != descriptor.File.ActualSize {
		_ = stored.Close()
		finish()
		if !replay {
			_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
		}
		return nil, errx.New(errc.ErrFileStorage, nil)
	}
	if _, err := stored.Seek(rangeSpec.Start, io.SeekStart); err != nil {
		_ = stored.Close()
		finish()
		if !replay {
			_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
		}
		return nil, errx.Wrap(errc.ErrFileStorage, err, nil)
	}
	if !replay {
		if err := createFileEvent(s.db, descriptor.File.RoomID, descriptor.File.ID, descriptor.File.BatchID, userID, FileEventDownloadStarted, now); err != nil {
			_ = stored.Close()
			finish()
			_ = s.db.Model(&model.DownloadTask{}).Where("id = ? AND status = ?", task.ID, model.DownloadTaskStreaming).Updates(map[string]interface{}{"status": model.DownloadTaskFailed, "failed_at": now}).Error
			return nil, err
		}
		s.publishFileProjection(descriptor.File.ID, "file.download_started", map[string]interface{}{"downloaderUserId": userID})
	}
	return &DownloadStream{svc: s, task: task, fileRecord: descriptor.File, file: stored, fileName: descriptor.FileName, mime: descriptor.MIME, finish: finish, taskContext: taskContext, rangeSpec: rangeSpec, hasRange: hasRange, replay: replay, allowComplete: !replay && (!hasRange || rangeSpec.End == descriptor.Size-1)}, nil
}

func (stream *DownloadStream) FileName() string { return stream.fileName }
func (stream *DownloadStream) MIME() string     { return stream.mime }
func (stream *DownloadStream) Size() int64      { return stream.rangeSpec.Length() }
func (stream *DownloadStream) TotalSize() int64 { return stream.task.ExpectedSize }

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
	remaining := stream.rangeSpec.Length()
	lastProgress := 0
	lastPublished := time.Time{}
	for remaining > 0 {
		if err := stream.taskContext.Err(); err != nil {
			stream.fail(transferred)
			return err
		}
		if err := ctx.Err(); err != nil {
			stream.fail(transferred)
			return err
		}
		chunk := buffer
		if int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		read, readErr := stream.file.Read(chunk)
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
			progress := 0
			if stream.task.ExpectedSize > 0 {
				progress = int((stream.rangeSpec.Start + transferred) * 100 / stream.task.ExpectedSize)
			}
			now := stream.svc.now()
			remaining -= int64(read)
			if !stream.replay && progress > lastProgress && (progress == 100 || now.Sub(lastPublished) >= 250*time.Millisecond) {
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
	if transferred != stream.rangeSpec.Length() {
		stream.fail(transferred)
		return io.ErrUnexpectedEOF
	}
	if stream.allowComplete {
		if err := stream.complete(stream.task.ExpectedSize); err != nil {
			stream.fail(transferred)
			return err
		}
		stream.svc.publishFileProjection(stream.fileRecord.ID, "file.download_completed", map[string]interface{}{"downloaderUserId": stream.task.UserID})
	}
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
			var task model.DownloadTask
			if err := tx.Where("id = ?", stream.task.ID).First(&task).Error; err != nil {
				return err
			}
			if task.Status == model.DownloadTaskCompleted {
				return nil
			}
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
	if stream.replay {
		return
	}
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
	if len(r.active) >= DownloadGlobalLimit || r.rooms[roomID] >= DownloadRoomLimit || r.users[userID] >= DownloadUserLimit {
		return nil, nil, errx.New(errc.ErrDownloadLimited, nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.next++
	key := fmt.Sprintf("%s:%d", taskID, r.next)
	r.active[key] = downloadActive{taskID: taskID, roomID: roomID, fileID: fileID, userID: userID, cancel: cancel}
	r.rooms[roomID]++
	r.users[userID]++
	var once sync.Once
	return ctx, func() {
		once.Do(func() {
			r.mu.Lock()
			delete(r.active, key)
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
