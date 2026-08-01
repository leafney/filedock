package service

import (
	"github.com/leafney/filedock/internal/model"
)

// publishFileProjection computes a distinct payload for every active member.
// It must never broadcast one participant's full private projection to a room.
func (s *FileSvc) publishFileProjection(fileID, eventType string, extra map[string]interface{}) {
	if s == nil || s.db == nil || s.hub == nil || fileID == "" || eventType == "" {
		return
	}
	var file model.RoomFile
	if err := s.db.Where("id = ?", fileID).First(&file).Error; err != nil {
		return
	}
	var room model.Room
	if err := s.db.Select("code").Where("id = ?", file.RoomID).First(&room).Error; err != nil {
		return
	}
	var members []model.RoomMember
	if err := s.db.Where("room_id = ? AND status = ?", file.RoomID, model.MemberStatusActive).Find(&members).Error; err != nil {
		return
	}
	for _, member := range members {
		projections, err := s.projectFiles([]model.RoomFile{file}, member)
		if err != nil || len(projections) == 0 {
			continue
		}
		payload := make(map[string]interface{}, len(extra)+2)
		payload["roomCode"] = room.Code
		payload["file"] = projections[0]
		for key, value := range extra {
			payload[key] = value
		}
		s.hub.PublishUser(member.UserID, eventType, payload)
	}
}
