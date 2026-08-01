package service

import (
	"encoding/base64"
	"errors"
	"sort"
	"strings"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"github.com/leafney/filedock/pkg/ulidx"
	"gorm.io/gorm"
)

const (
	FileRangeAll     = "all"
	FileIdentityAll  = "all"
	FileIdentityMine = "uploaded"
	FileIdentityToMe = "received"
	FileSortNewest   = "newest"
	FileSortOldest   = "oldest"
	FileSortSizeAsc  = "size_asc"
	FileSortSizeDesc = "size_desc"
	DefaultFileLimit = 30
	MaximumFileLimit = 100
)

type FileListQuery struct {
	Range        string
	Identity     string
	Search       string
	Sort         string
	Limit        int
	SharedCursor string
	DirectCursor string
}

type FileGroupResult struct {
	Scope      string           `json:"scope"`
	Items      []FileProjection `json:"items"`
	Total      int              `json:"total"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

type FileListResult struct {
	Shared *FileGroupResult `json:"shared,omitempty"`
	Direct *FileGroupResult `json:"direct,omitempty"`
}

type FileEventProjection struct {
	EventID   string          `json:"eventId"`
	Type      string          `json:"type"`
	ActorID   string          `json:"actorId,omitempty"`
	ActorName string          `json:"actorName,omitempty"`
	CreatedAt int64           `json:"createdAt"`
	File      *FileProjection `json:"file,omitempty"`
}

type FileEventPage struct {
	Items      []FileEventProjection `json:"items"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

type ReuseResult struct {
	Changed int `json:"changed"`
	Skipped int `json:"skipped"`
}

func (s *FileSvc) ListFiles(userID, roomCode string, query FileListQuery) (FileListResult, error) {
	room, viewer, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return FileListResult{}, err
	}
	normalizeFileListQuery(&query)
	var files []model.RoomFile
	dbQuery := s.db.Where("room_id = ? AND status IN ?", room.ID, []string{model.FileStatusUploading, model.FileStatusAvailable})
	if query.Range == model.FileScopeShared || query.Range == model.FileScopeDirect {
		dbQuery = dbQuery.Where("scope = ?", query.Range)
	}
	if query.Identity == FileIdentityMine {
		dbQuery = dbQuery.Where("uploader_user_id = ?", userID)
	} else if query.Identity == FileIdentityToMe {
		dbQuery = dbQuery.Where("id IN (?)", s.db.Model(&model.FileRecipient{}).Select("file_id").Where("recipient_user_id = ?", userID))
	}
	if err := dbQuery.Find(&files).Error; err != nil {
		return FileListResult{}, err
	}
	projections, err := s.projectFiles(files, viewer)
	if err != nil {
		return FileListResult{}, err
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	filtered := projections[:0]
	for _, projection := range projections {
		if search == "" || strings.Contains(strings.ToLower(projection.DisplayName), search) || (projection.Level == ProjectionFull && strings.Contains(strings.ToLower(projection.OriginalName), search)) {
			filtered = append(filtered, projection)
		}
	}
	sortFileProjections(filtered, query.Sort)
	result := FileListResult{}
	if query.Range == FileRangeAll || query.Range == model.FileScopeShared {
		group := paginateFileGroup(filtered, model.FileScopeShared, query.SharedCursor, query.Limit)
		result.Shared = &group
	}
	if query.Range == FileRangeAll || query.Range == model.FileScopeDirect {
		group := paginateFileGroup(filtered, model.FileScopeDirect, query.DirectCursor, query.Limit)
		result.Direct = &group
	}
	return result, nil
}

func (s *FileSvc) ReusableFiles(userID, roomCode string) ([]FileProjection, error) {
	room, viewer, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return nil, err
	}
	var files []model.RoomFile
	if err := s.db.Where("room_id = ? AND uploader_user_id = ? AND scope = ? AND status = ?", room.ID, userID, model.FileScopeDirect, model.FileStatusAvailable).Order("completed_at DESC, id DESC").Find(&files).Error; err != nil {
		return nil, err
	}
	return s.projectFiles(files, viewer)
}

func (s *FileSvc) ListFileEvents(userID, roomCode, cursor string, limit int) (FileEventPage, error) {
	room, viewer, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return FileEventPage{}, err
	}
	limit = normalizeLimit(limit)
	var events []model.FileEvent
	if err := s.db.Where("room_id = ?", room.ID).Order("created_at DESC, id DESC").Find(&events).Error; err != nil {
		return FileEventPage{}, err
	}
	start := cursorStart(events, cursor, func(event model.FileEvent) string { return event.ID })
	items := make([]FileEventProjection, 0, limit)
	next := ""
	for index := start; index < len(events); index++ {
		event := events[index]
		projection, visible, err := s.projectEvent(event, viewer)
		if err != nil {
			return FileEventPage{}, err
		}
		if !visible {
			continue
		}
		if len(items) == limit {
			next = encodeCursor(items[len(items)-1].EventID)
			break
		}
		items = append(items, projection)
	}
	return FileEventPage{Items: items, NextCursor: next}, nil
}

func (s *FileSvc) AcceptFile(userID, roomCode, fileID string) error {
	return s.changeRecipientStatus(userID, roomCode, fileID, model.RecipientPending, model.RecipientAccepted, FileEventAccepted)
}

func (s *FileSvc) DeclineFile(userID, roomCode, fileID string) error {
	return s.changeRecipientStatus(userID, roomCode, fileID, model.RecipientPending, model.RecipientDeclined, FileEventDeclined)
}

func (s *FileSvc) changeRecipientStatus(userID, roomCode, fileID, from, to, eventType string) error {
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return err
	}
	now := s.now().Unix()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var file model.RoomFile
		if err := tx.Where("id = ? AND room_id = ? AND scope = ? AND status = ?", fileID, room.ID, model.FileScopeDirect, model.FileStatusAvailable).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		updates := map[string]interface{}{"status": to}
		if to == model.RecipientAccepted {
			updates["accepted_at"] = now
		} else {
			updates["declined_at"] = now
		}
		result := tx.Model(&model.FileRecipient{}).Where("file_id = ? AND recipient_user_id = ? AND status = ?", file.ID, userID, from).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errx.New(errc.ErrFileRecipientState, nil)
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, eventType, now)
	})
	if err == nil {
		s.publishFileProjection(fileID, "file.recipient_changed", map[string]interface{}{"recipientUserId": userID, "status": to})
	}
	return err
}

func (s *FileSvc) ReusePrivateFiles(userID, roomCode string, fileIDs, recipientIDs []string) (ReuseResult, error) {
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return ReuseResult{}, err
	}
	if len(fileIDs) == 0 {
		return ReuseResult{}, errx.New(errc.ErrFileManifest, nil)
	}
	var result ReuseResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		recipients, err := validateRecipients(tx, room.ID, userID, model.FileScopeDirect, recipientIDs)
		if err != nil {
			return err
		}
		uniqueFiles := uniqueStrings(fileIDs)
		if len(uniqueFiles) != len(fileIDs) {
			return errx.New(errc.ErrFileManifest, nil)
		}
		var files []model.RoomFile
		if err := tx.Where("id IN ? AND room_id = ? AND uploader_user_id = ? AND scope = ? AND status = ?", uniqueFiles, room.ID, userID, model.FileScopeDirect, model.FileStatusAvailable).Find(&files).Error; err != nil {
			return err
		}
		if len(files) != len(uniqueFiles) {
			return errx.New(errc.ErrFileNotFound, nil)
		}
		now := s.now().Unix()
		for _, file := range files {
			fileChanged := false
			for _, recipient := range recipients {
				var relation model.FileRecipient
				err := tx.Where("file_id = ? AND recipient_user_id = ?", file.ID, recipient.UserID).First(&relation).Error
				switch {
				case errors.Is(err, gorm.ErrRecordNotFound):
					id, idErr := ulidx.New()
					if idErr != nil {
						return idErr
					}
					if err := tx.Create(&model.FileRecipient{ID: id, FileID: file.ID, RecipientUserID: recipient.UserID, Status: model.RecipientPending, SentAt: now}).Error; err != nil {
						return err
					}
					result.Changed++
					fileChanged = true
				case err != nil:
					return err
				case relation.Status == model.RecipientDeclined:
					if err := tx.Model(&model.FileRecipient{}).Where("id = ? AND status = ?", relation.ID, model.RecipientDeclined).Updates(map[string]interface{}{"status": model.RecipientPending, "sent_at": now, "accepted_at": nil, "declined_at": nil}).Error; err != nil {
						return err
					}
					result.Changed++
					fileChanged = true
				default:
					result.Skipped++
				}
			}
			if fileChanged {
				if err := createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventReused, now); err != nil {
					return err
				}
			}
		}
		if result.Changed == 0 {
			return errx.New(errc.ErrFileRecipientState, nil)
		}
		return nil
	})
	if err == nil {
		for _, fileID := range fileIDs {
			s.publishFileProjection(fileID, "file.reused", nil)
		}
	}
	return result, err
}

func (s *FileSvc) PublishShared(userID, roomCode, fileID string) error {
	room, _, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return err
	}
	now := s.now().Unix()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var file model.RoomFile
		if err := tx.Where("id = ? AND room_id = ? AND uploader_user_id = ? AND scope = ? AND status = ?", fileID, room.ID, userID, model.FileScopeDirect, model.FileStatusAvailable).First(&file).Error; err != nil {
			return fileNotFound(err)
		}
		result := tx.Model(&model.RoomFile{}).Where("id = ? AND scope = ?", file.ID, model.FileScopeDirect).Update("scope", model.FileScopeShared)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errx.New(errc.ErrFileState, nil)
		}
		return createFileEvent(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventPublished, now)
	})
	if err == nil {
		s.publishFileProjection(fileID, "file.published_shared", nil)
	}
	return err
}

func (s *FileSvc) projectFiles(files []model.RoomFile, viewer model.RoomMember) ([]FileProjection, error) {
	result := make([]FileProjection, 0, len(files))
	for _, file := range files {
		var uploader model.RoomMember
		if err := s.db.Where("room_id = ? AND user_id = ?", file.RoomID, file.UploaderUserID).First(&uploader).Error; err != nil {
			return nil, err
		}
		var recipients []model.FileRecipient
		if err := s.db.Where("file_id = ?", file.ID).Order("sent_at ASC, id ASC").Find(&recipients).Error; err != nil {
			return nil, err
		}
		members := make(map[string]model.RoomMember, len(recipients))
		for _, recipient := range recipients {
			var member model.RoomMember
			if err := s.db.Where("room_id = ? AND user_id = ?", file.RoomID, recipient.RecipientUserID).First(&member).Error; err == nil {
				members[recipient.RecipientUserID] = member
			}
		}
		projection := ProjectFile(file, uploader, recipients, members, viewer)
		if projection.Level != ProjectionHidden {
			result = append(result, projection)
		}
	}
	return result, nil
}

func (s *FileSvc) projectEvent(event model.FileEvent, viewer model.RoomMember) (FileEventProjection, bool, error) {
	projection := FileEventProjection{EventID: event.ID, Type: event.Type, ActorID: event.ActorUserID, CreatedAt: event.CreatedAt}
	if event.ActorUserID != "" {
		var actor model.RoomMember
		if err := s.db.Where("room_id = ? AND user_id = ?", event.RoomID, event.ActorUserID).First(&actor).Error; err == nil {
			projection.ActorName = actor.DisplayName
		}
	}
	if event.FileID == "" {
		return projection, event.ActorUserID == viewer.UserID || viewer.Role == model.MemberRoleOwner, nil
	}
	var file model.RoomFile
	if err := s.db.Where("id = ? AND room_id = ?", event.FileID, event.RoomID).First(&file).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return projection, false, nil
		}
		return projection, false, err
	}
	files, err := s.projectFiles([]model.RoomFile{file}, viewer)
	if err != nil || len(files) == 0 {
		return projection, false, err
	}
	projection.File = &files[0]
	return projection, true, nil
}

func normalizeFileListQuery(query *FileListQuery) {
	if query.Range != model.FileScopeShared && query.Range != model.FileScopeDirect {
		query.Range = FileRangeAll
	}
	if query.Identity != FileIdentityMine && query.Identity != FileIdentityToMe {
		query.Identity = FileIdentityAll
	}
	if query.Sort != FileSortOldest && query.Sort != FileSortSizeAsc && query.Sort != FileSortSizeDesc {
		query.Sort = FileSortNewest
	}
	query.Limit = normalizeLimit(query.Limit)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultFileLimit
	}
	if limit > MaximumFileLimit {
		return MaximumFileLimit
	}
	return limit
}

func sortFileProjections(files []FileProjection, order string) {
	sort.SliceStable(files, func(i, j int) bool {
		left, right := files[i], files[j]
		switch order {
		case FileSortOldest:
			if left.CreatedAt != right.CreatedAt {
				return left.CreatedAt < right.CreatedAt
			}
		case FileSortSizeAsc:
			if left.Size != right.Size {
				return left.Size < right.Size
			}
		case FileSortSizeDesc:
			if left.Size != right.Size {
				return left.Size > right.Size
			}
		default:
			if left.CreatedAt != right.CreatedAt {
				return left.CreatedAt > right.CreatedAt
			}
		}
		return left.FileID > right.FileID
	})
}

func paginateFileGroup(files []FileProjection, scope, cursor string, limit int) FileGroupResult {
	items := make([]FileProjection, 0)
	for _, file := range files {
		if file.Scope == scope {
			items = append(items, file)
		}
	}
	total := len(items)
	start := cursorStart(items, cursor, func(file FileProjection) string { return file.FileID })
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	next := ""
	if end < len(items) {
		next = encodeCursor(items[end-1].FileID)
	} else {
		end = len(items)
	}
	return FileGroupResult{Scope: scope, Items: items[start:end], Total: total, NextCursor: next}
}

func encodeCursor(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }

func decodeCursor(value string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func cursorStart[T any](items []T, cursor string, id func(T) string) int {
	decoded := decodeCursor(cursor)
	if decoded == "" {
		return 0
	}
	for index, item := range items {
		if id(item) == decoded {
			return index + 1
		}
	}
	return 0
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
