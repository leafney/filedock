package service

import (
	"encoding/base64"
	"encoding/json"
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
	EventID           string                     `json:"eventId"`
	OperationID       string                     `json:"operationId"`
	OperationType     string                     `json:"operationType"`
	Type              string                     `json:"type"`
	ActorID           string                     `json:"actorId,omitempty"`
	ActorName         string                     `json:"actorName,omitempty"`
	CreatedAt         int64                      `json:"createdAt"`
	File              *FileProjection            `json:"file,omitempty"`
	History           []FileEventHistory         `json:"history"`
	RecipientSummary  *FileRecipientEventSummary `json:"recipientSummary,omitempty"`
	SkippedRecipients []FileEventRecipient       `json:"skippedRecipients,omitempty"`
}

type FileEventHistory struct {
	EventID    string               `json:"eventId"`
	Type       string               `json:"type"`
	ActorID    string               `json:"actorId,omitempty"`
	ActorName  string               `json:"actorName,omitempty"`
	CreatedAt  int64                `json:"createdAt"`
	Recipients []FileEventRecipient `json:"recipients,omitempty"`
}

type FileEventRecipient struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status,omitempty"`
}

type FileRecipientEventSummary struct {
	Total    int `json:"total"`
	Pending  int `json:"pending"`
	Accepted int `json:"accepted"`
	Declined int `json:"declined"`
	Skipped  int `json:"skipped,omitempty"`
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
	if err := s.db.Where("room_id = ? AND file_id <> ''", room.ID).Order("created_at DESC, id DESC").Find(&events).Error; err != nil {
		return FileEventPage{}, err
	}
	groups := make(map[string][]model.FileEvent)
	order := make([]string, 0)
	for _, event := range events {
		operationID := event.OperationID
		if operationID == "" {
			operationID = event.FileID
		}
		key := event.FileID + "\x00" + operationID
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], event)
	}
	cards := make([]FileEventProjection, 0, len(order))
	for _, key := range order {
		card, visible, projectErr := s.projectEventGroup(groups[key], viewer)
		if projectErr != nil {
			return FileEventPage{}, projectErr
		}
		if visible {
			cards = append(cards, card)
		}
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].CreatedAt != cards[j].CreatedAt {
			return cards[i].CreatedAt > cards[j].CreatedAt
		}
		return cards[i].EventID > cards[j].EventID
	})
	start := cursorStart(cards, cursor, func(card FileEventProjection) string { return card.EventID })
	if start > len(cards) {
		start = len(cards)
	}
	end := start + limit
	next := ""
	if end < len(cards) {
		next = encodeCursor(cards[end-1].EventID)
	} else {
		end = len(cards)
	}
	return FileEventPage{Items: cards[start:end], NextCursor: next}, nil
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
		var recipient model.FileRecipient
		if err := tx.Where("file_id = ? AND recipient_user_id = ? AND status = ?", file.ID, userID, from).First(&recipient).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errx.New(errc.ErrFileRecipientState, nil)
			}
			return err
		}
		updates := map[string]interface{}{"status": to}
		if to == model.RecipientAccepted {
			updates["accepted_at"] = now
		} else {
			updates["declined_at"] = now
		}
		result := tx.Model(&model.FileRecipient{}).Where("id = ? AND status = ? AND delivery_version = ?", recipient.ID, from, recipient.DeliveryVersion).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errx.New(errc.ErrFileRecipientState, nil)
		}
		event, err := createFileEventRecordWithOperation(tx, file.RoomID, file.ID, file.BatchID, userID, eventType, recipient.OperationID, now, fileEventPayload{RecipientUserID: userID, DeliveryVersion: recipient.DeliveryVersion})
		if err != nil {
			return err
		}
		if to == model.RecipientDeclined {
			return s.recordFileNotification(tx, NotificationTypeFileDeclined, file.UploaderUserID, userID, file, recipient, event)
		}
		return nil
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
			changedRecipients := make([]model.FileRecipient, 0, len(recipients))
			skippedRecipientIDs := make([]string, 0)
			reuseOperationID, idErr := ulidx.New()
			if idErr != nil {
				return idErr
			}
			for _, recipient := range recipients {
				var relation model.FileRecipient
				err := tx.Where("file_id = ? AND recipient_user_id = ?", file.ID, recipient.UserID).First(&relation).Error
				switch {
				case errors.Is(err, gorm.ErrRecordNotFound):
					id, idErr := ulidx.New()
					if idErr != nil {
						return idErr
					}
					relation = model.FileRecipient{ID: id, FileID: file.ID, RecipientUserID: recipient.UserID, OperationID: reuseOperationID, DeliveryVersion: 1, Status: model.RecipientPending, SentAt: now}
					if err := tx.Create(&relation).Error; err != nil {
						return err
					}
					result.Changed++
					changedRecipients = append(changedRecipients, relation)
				case err != nil:
					return err
				case relation.Status == model.RecipientDeclined:
					updates := map[string]interface{}{
						"status":              model.RecipientPending,
						"delivery_version":    gorm.Expr("delivery_version + 1"),
						"sent_at":             now,
						"accepted_at":         nil,
						"declined_at":         nil,
						"first_downloaded_at": nil,
						"last_downloaded_at":  nil,
						"download_count":      0,
					}
					update := tx.Model(&model.FileRecipient{}).Where("id = ? AND status = ? AND delivery_version = ?", relation.ID, model.RecipientDeclined, relation.DeliveryVersion).Updates(updates)
					if update.Error != nil {
						return update.Error
					}
					if update.RowsAffected != 1 {
						return errx.New(errc.ErrFileRecipientState, nil)
					}
					if err := tx.First(&relation, "id = ?", relation.ID).Error; err != nil {
						return err
					}
					if err := tx.Model(&model.FileRecipient{}).Where("id = ?", relation.ID).Update("operation_id", reuseOperationID).Error; err != nil {
						return err
					}
					relation.OperationID = reuseOperationID
					result.Changed++
					changedRecipients = append(changedRecipients, relation)
				default:
					result.Skipped++
					skippedRecipientIDs = append(skippedRecipientIDs, recipient.UserID)
				}
			}
			if len(changedRecipients) > 0 {
				changedRecipientIDs := make([]string, 0, len(changedRecipients))
				for _, relation := range changedRecipients {
					changedRecipientIDs = append(changedRecipientIDs, relation.RecipientUserID)
				}
				event, err := createFileEventRecordWithOperation(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventReused, reuseOperationID, now, fileEventPayload{RecipientIDs: changedRecipientIDs, SkippedRecipientIDs: skippedRecipientIDs})
				if err != nil {
					return err
				}
				for _, relation := range changedRecipients {
					if err := s.recordFileNotification(tx, NotificationTypeFileReceived, relation.RecipientUserID, file.UploaderUserID, file, relation, event); err != nil {
						return err
					}
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
	operationID, err := ulidx.New()
	if err != nil {
		return err
	}
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
		return createFileEventWithOperation(tx, file.RoomID, file.ID, file.BatchID, userID, FileEventPublished, operationID, now, nil)
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

func (s *FileSvc) projectEventGroup(events []model.FileEvent, viewer model.RoomMember) (FileEventProjection, bool, error) {
	if len(events) == 0 {
		return FileEventProjection{}, false, nil
	}
	latest := events[0]
	operationID := latest.OperationID
	if operationID == "" {
		operationID = latest.FileID
	}
	projection := FileEventProjection{EventID: latest.ID, OperationID: operationID, OperationType: fileEventOperationType(events), Type: latest.Type, ActorID: latest.ActorUserID, CreatedAt: latest.CreatedAt, History: make([]FileEventHistory, 0, len(events))}
	if latest.ActorUserID != "" {
		var actor model.RoomMember
		if err := s.db.Where("room_id = ? AND user_id = ?", latest.RoomID, latest.ActorUserID).First(&actor).Error; err == nil {
			projection.ActorName = actor.DisplayName
		}
	}
	var file model.RoomFile
	fileExists := s.db.Where("id = ? AND room_id = ?", latest.FileID, latest.RoomID).First(&file).Error == nil
	if !fileExists {
		return projection, viewer.Role == model.MemberRoleOwner || latest.ActorUserID == viewer.UserID, nil
	}
	var recipients []model.FileRecipient
	if err := s.db.Where("file_id = ?", file.ID).Order("sent_at ASC, id ASC").Find(&recipients).Error; err != nil {
		return FileEventProjection{}, false, err
	}
	recipientMembers := make(map[string]model.RoomMember, len(recipients))
	for _, recipient := range recipients {
		var member model.RoomMember
		if err := s.db.Where("room_id = ? AND user_id = ?", file.RoomID, recipient.RecipientUserID).First(&member).Error; err == nil {
			recipientMembers[recipient.RecipientUserID] = member
		}
	}
	payloads := make([]fileEventPayload, len(events))
	operationRecipientIDs := make(map[string]struct{})
	operationRecipientStatus := make(map[string]string)
	skippedRecipientIDs := make(map[string]struct{})
	for index, event := range events {
		payloads[index] = decodeFileEventPayload(event)
		payload := payloads[index]
		for _, recipientID := range payload.RecipientIDs {
			operationRecipientIDs[recipientID] = struct{}{}
			if _, exists := operationRecipientStatus[recipientID]; !exists {
				operationRecipientStatus[recipientID] = model.RecipientPending
			}
		}
		for _, recipientID := range payload.SkippedRecipientIDs {
			skippedRecipientIDs[recipientID] = struct{}{}
		}
		if payload.RecipientUserID != "" {
			operationRecipientIDs[payload.RecipientUserID] = struct{}{}
			switch event.Type {
			case FileEventAccepted:
				operationRecipientStatus[payload.RecipientUserID] = model.RecipientAccepted
			case FileEventDeclined:
				operationRecipientStatus[payload.RecipientUserID] = model.RecipientDeclined
			}
		}
	}
	participant := file.UploaderUserID == viewer.UserID
	viewerRecipient := ""
	for recipientID := range operationRecipientIDs {
		if recipientID == viewer.UserID {
			participant = true
			viewerRecipient = recipientID
			break
		}
	}
	if file.Scope == model.FileScopeDirect && !participant && viewer.Role != model.MemberRoleOwner {
		return projection, false, nil
	}
	files, err := s.projectFiles([]model.RoomFile{file}, viewer)
	if err != nil {
		return FileEventProjection{}, false, err
	}
	if len(files) == 0 {
		return projection, false, nil
	}
	projection.File = &files[0]
	if file.Scope == model.FileScopeDirect {
		if viewer.Role == model.MemberRoleOwner && !participant {
			projection.File.Recipients = nil
		} else if viewerRecipient != "" && viewerRecipient != file.UploaderUserID {
			filtered := make([]FileRecipientView, 0, 1)
			for _, recipient := range projection.File.Recipients {
				if recipient.UserID == viewerRecipient {
					filtered = append(filtered, recipient)
				}
			}
			projection.File.Recipients = filtered
		}
		if len(operationRecipientIDs) > 0 {
			summary := &FileRecipientEventSummary{Total: len(operationRecipientIDs), Skipped: len(skippedRecipientIDs)}
			for recipientID := range operationRecipientIDs {
				switch operationRecipientStatus[recipientID] {
				case model.RecipientAccepted, model.RecipientDownloaded:
					summary.Accepted++
				case model.RecipientDeclined:
					summary.Declined++
				default:
					summary.Pending++
				}
			}
			projection.RecipientSummary = summary
		}
	}
	for index, event := range events {
		history := FileEventHistory{EventID: event.ID, Type: event.Type, ActorID: event.ActorUserID, CreatedAt: event.CreatedAt}
		if event.ActorUserID != "" {
			var actor model.RoomMember
			if err := s.db.Where("room_id = ? AND user_id = ?", event.RoomID, event.ActorUserID).First(&actor).Error; err == nil {
				history.ActorName = actor.DisplayName
			}
		}
		if file.Scope == model.FileScopeDirect && (viewer.UserID == file.UploaderUserID || viewerRecipient != "") {
			payload := payloads[index]
			for _, recipientID := range payload.RecipientIDs {
				if viewerRecipient != "" && recipientID != viewerRecipient {
					continue
				}
				history.Recipients = append(history.Recipients, projectEventRecipient(recipientID, operationRecipientStatus[recipientID], recipientMembers))
			}
			for _, recipientID := range payload.SkippedRecipientIDs {
				if viewerRecipient != "" && recipientID != viewerRecipient {
					continue
				}
				history.Recipients = append(history.Recipients, projectEventRecipient(recipientID, "skipped", recipientMembers))
			}
			if payload.RecipientUserID != "" && (viewerRecipient == "" || payload.RecipientUserID == viewerRecipient) {
				history.Recipients = append(history.Recipients, projectEventRecipient(payload.RecipientUserID, operationRecipientStatus[payload.RecipientUserID], recipientMembers))
			}
		}
		projection.History = append(projection.History, history)
	}
	for recipientID := range skippedRecipientIDs {
		if viewer.UserID == file.UploaderUserID {
			projection.SkippedRecipients = append(projection.SkippedRecipients, projectEventRecipient(recipientID, "skipped", recipientMembers))
		}
	}
	return projection, true, nil
}

func decodeFileEventPayload(event model.FileEvent) fileEventPayload {
	var payload fileEventPayload
	if event.PayloadJSON != "" {
		_ = json.Unmarshal([]byte(event.PayloadJSON), &payload)
	}
	return payload
}

func projectEventRecipient(userID, status string, members map[string]model.RoomMember) FileEventRecipient {
	name := userID
	if member, ok := members[userID]; ok {
		name = member.DisplayName
	}
	return FileEventRecipient{UserID: userID, DisplayName: name, Status: status}
}

func fileEventOperationType(events []model.FileEvent) string {
	for _, event := range events {
		switch event.Type {
		case FileEventReused:
			return "transfer"
		case FileEventDownloadStarted, FileEventDownloaded, FileEventDownloadFailed, FileEventDownloadCancelled:
			return "download"
		case FileEventTrashed:
			return "trash"
		case FileEventRestoreRequested, FileEventRestoreRejected, FileEventRestored:
			return "restore"
		case FileEventPurged:
			return "purge"
		case FileEventPublished:
			return "publish"
		}
	}
	return "upload"
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
