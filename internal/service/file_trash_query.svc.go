package service

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/leafney/filedock/internal/model"
	"gorm.io/gorm"
)

type FileTrashListQuery struct {
	Search string
	Cursor string
	Limit  int
}

type FileTrashCapabilities struct {
	CanRestore        bool `json:"canRestore"`
	CanRequestRestore bool `json:"canRequestRestore"`
	CanPurge          bool `json:"canPurge"`
}

type FileRestoreRequestProjection struct {
	RequestID       string `json:"requestId"`
	Status          string `json:"status"`
	RequesterUserID string `json:"requesterUserId"`
	RequesterName   string `json:"requesterName"`
	CreatedAt       int64  `json:"createdAt"`
	RejectionReason string `json:"rejectionReason,omitempty"`
}

type FileTrashProjection struct {
	File           FileProjection                `json:"file"`
	DeletedByID    string                        `json:"deletedByUserId"`
	DeletedByName  string                        `json:"deletedByName"`
	DeletedAt      int64                         `json:"deletedAt"`
	DeleteReason   string                        `json:"deleteReason,omitempty"`
	RestoreRequest *FileRestoreRequestProjection `json:"restoreRequest,omitempty"`
	Capabilities   FileTrashCapabilities         `json:"capabilities"`
}

type FileTrashPage struct {
	Items      []FileTrashProjection `json:"items"`
	Total      int                   `json:"total"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

// ListTrash projects every row before applying search so anonymous private
// filenames can never influence a viewer's result set or total count.
func (s *FileSvc) ListTrash(userID, roomCode string, query FileTrashListQuery) (FileTrashPage, error) {
	room, viewer, err := activeFileMember(s.db, roomCode, userID)
	if err != nil {
		return FileTrashPage{}, err
	}
	query.Search = strings.ToLower(strings.TrimSpace(query.Search))
	query.Limit = normalizeLimit(query.Limit)

	filesQuery := s.db.Where("room_id = ? AND status = ?", room.ID, model.FileStatusTrashed)
	if viewer.Role != model.MemberRoleOwner {
		filesQuery = filesQuery.Where("uploader_user_id = ?", userID)
	}
	var files []model.RoomFile
	if err := filesQuery.Find(&files).Error; err != nil {
		return FileTrashPage{}, err
	}
	items := make([]FileTrashProjection, 0, len(files))
	for _, file := range files {
		projected, err := s.projectTrashFile(file, viewer)
		if err != nil {
			return FileTrashPage{}, err
		}
		if projected.File.Level == ProjectionHidden || !trashProjectionMatches(projected.File, query.Search) {
			continue
		}
		items = append(items, projected)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DeletedAt != items[j].DeletedAt {
			return items[i].DeletedAt > items[j].DeletedAt
		}
		return items[i].File.FileID > items[j].File.FileID
	})
	return paginateTrash(items, query.Cursor, query.Limit), nil
}

func (s *FileSvc) projectTrashFile(file model.RoomFile, viewer model.RoomMember) (FileTrashProjection, error) {
	files, err := s.projectFiles([]model.RoomFile{file}, viewer)
	if err != nil || len(files) == 0 {
		return FileTrashProjection{}, err
	}
	var cycle model.FileTrashCycle
	if err := s.db.Where("file_id = ? AND version = ? AND outcome = ?", file.ID, file.TrashVersion, model.FileTrashActive).First(&cycle).Error; err != nil {
		return FileTrashProjection{}, err
	}
	projection := FileTrashProjection{
		File:          files[0],
		DeletedByID:   cycle.DeletedByUserID,
		DeletedByName: s.fileMemberName(file.RoomID, cycle.DeletedByUserID),
		DeletedAt:     cycle.DeletedAt,
		DeleteReason:  cycle.DeleteReason,
	}
	var request model.FileRestoreRequest
	requestErr := s.db.Where("trash_cycle_id = ?", cycle.ID).First(&request).Error
	if requestErr == nil {
		projection.RestoreRequest = &FileRestoreRequestProjection{
			RequestID:       request.ID,
			Status:          request.Status,
			RequesterUserID: request.RequesterUserID,
			RequesterName:   s.fileMemberName(file.RoomID, request.RequesterUserID),
			CreatedAt:       request.CreatedAt,
			RejectionReason: request.RejectionReason,
		}
	} else if !errors.Is(requestErr, gorm.ErrRecordNotFound) {
		return FileTrashProjection{}, requestErr
	}

	if viewer.Role == model.MemberRoleOwner {
		projection.Capabilities.CanRestore = true
		projection.Capabilities.CanPurge = true
		return projection, nil
	}
	if file.UploaderUserID != viewer.UserID {
		return FileTrashProjection{}, nil
	}
	if cycle.DeletedByUserID == viewer.UserID {
		projection.Capabilities.CanRestore = true
		projection.Capabilities.CanPurge = true
		return projection, nil
	}
	projection.Capabilities.CanRequestRestore = projection.RestoreRequest == nil || projection.RestoreRequest.Status == model.FileRestoreInvalidated
	return projection, nil
}

func (s *FileSvc) fileMemberName(roomID, userID string) string {
	if userID == "" {
		return ""
	}
	var member model.RoomMember
	if err := s.db.Select("display_name").Where("room_id = ? AND user_id = ?", roomID, userID).First(&member).Error; err != nil {
		return ""
	}
	return member.DisplayName
}

func trashProjectionMatches(file FileProjection, search string) bool {
	if search == "" {
		return true
	}
	if strings.Contains(strings.ToLower(file.DisplayName), search) {
		return true
	}
	if file.Level != ProjectionFull {
		return false
	}
	return strings.Contains(strings.ToLower(file.OriginalName), search) || strings.Contains(strings.ToLower(file.PrivateCode), search)
}

func paginateTrash(items []FileTrashProjection, cursor string, limit int) FileTrashPage {
	total := len(items)
	start := 0
	decoded := decodeCursor(cursor)
	if decoded != "" {
		for index, item := range items {
			if trashCursorKey(item) == decoded {
				start = index + 1
				break
			}
		}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := FileTrashPage{Items: items[start:end], Total: total}
	if end < len(items) && end > start {
		page.NextCursor = encodeCursor(trashCursorKey(items[end-1]))
	}
	return page
}

func trashCursorKey(item FileTrashProjection) string {
	return strconv.FormatInt(item.DeletedAt, 10) + ":" + item.File.FileID
}
