package biz

import (
	"fmt"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
)

type RoomBiz struct {
	room *service.RoomSvc
}

func NewRoomBiz(room *service.RoomSvc) (*RoomBiz, error) {
	if room == nil {
		return nil, fmt.Errorf("room service is required")
	}
	return &RoomBiz{room: room}, nil
}

func (b *RoomBiz) Create(userID string, request dto.CreateRoomRequest) (dto.RoomSnapshotDTO, error) {
	snapshot, err := b.room.Create(userID, request.JoinMode, request.PIN, request.PINConfirmation)
	return roomSnapshotDTO(snapshot), err
}

func (b *RoomBiz) List(userID string) ([]dto.RoomSummaryDTO, error) {
	items, err := b.room.List(userID)
	if err != nil {
		return nil, err
	}
	result := make([]dto.RoomSummaryDTO, 0, len(items))
	for _, item := range items {
		result = append(result, dto.RoomSummaryDTO{RoomID: item.RoomID, RoomCode: item.RoomCode, Title: item.Title, Status: item.Status, Role: item.Role, ExpiresAt: item.ExpiresAt})
	}
	return result, nil
}

func (b *RoomBiz) JoinInfo(userID, code string) (dto.RoomJoinInfoDTO, error) {
	info, err := b.room.JoinInfo(userID, code)
	return dto.RoomJoinInfoDTO{RoomCode: info.RoomCode, Title: info.Title, JoinMode: info.JoinMode, AlreadyMember: info.AlreadyMember, PendingRequest: info.PendingRequest}, err
}

func (b *RoomBiz) Join(userID, code string, request dto.JoinRoomRequest) (dto.RoomSnapshotDTO, error) {
	snapshot, err := b.room.Join(userID, code, request.Confirmed, request.PIN)
	return roomSnapshotDTO(snapshot), err
}

func (b *RoomBiz) Snapshot(userID, code string) (dto.RoomSnapshotDTO, error) {
	snapshot, err := b.room.Snapshot(userID, code)
	return roomSnapshotDTO(snapshot), err
}

func (b *RoomBiz) Members(userID, code string) ([]dto.RoomMemberDTO, error) {
	members, err := b.room.Members(userID, code)
	if err != nil {
		return nil, err
	}
	return roomMembersDTO(members), nil
}

func (b *RoomBiz) Extend(userID, code string) (dto.ExtendRoomDTO, error) {
	expiresAt, err := b.room.Extend(userID, code)
	return dto.ExtendRoomDTO{ExpiresAt: expiresAt}, err
}

func (b *RoomBiz) Leave(userID, code string) error { return b.room.Leave(userID, code) }

func (b *RoomBiz) Dissolve(userID, code string) error { return b.room.Dissolve(userID, code) }

func (b *RoomBiz) Kick(ownerID, code, targetUserID string) error {
	return b.room.Kick(ownerID, code, targetUserID)
}

func (b *RoomBiz) CreateJoinRequest(userID, code string) (dto.JoinRequestDTO, error) {
	request, err := b.room.CreateJoinRequest(userID, code)
	return joinRequestDTO(request), err
}

func (b *RoomBiz) ListJoinRequests(ownerID, code string) (dto.JoinRequestListDTO, error) {
	count, requests, err := b.room.ListJoinRequests(ownerID, code)
	if err != nil {
		return dto.JoinRequestListDTO{}, err
	}
	result := dto.JoinRequestListDTO{Items: make([]dto.JoinRequestDTO, 0, len(requests)), PendingCount: count}
	for _, request := range requests {
		result.Items = append(result.Items, joinRequestDTO(request))
	}
	return result, nil
}

func (b *RoomBiz) CancelJoinRequest(userID, code string) error {
	return b.room.CancelJoinRequest(userID, code)
}

func (b *RoomBiz) ApproveJoinRequest(ownerID, code, requestID string) error {
	return b.room.ApproveJoinRequest(ownerID, code, requestID)
}

func (b *RoomBiz) RejectJoinRequest(ownerID, code, requestID string) error {
	return b.room.RejectJoinRequest(ownerID, code, requestID)
}

func roomSnapshotDTO(snapshot service.RoomSnapshot) dto.RoomSnapshotDTO {
	return dto.RoomSnapshotDTO{RoomID: snapshot.RoomID, RoomCode: snapshot.RoomCode, Title: snapshot.Title, Status: snapshot.Status, JoinMode: snapshot.JoinMode, Role: snapshot.Role, ExpiresAt: snapshot.ExpiresAt, CanExtend: snapshot.CanExtend, DestroyAt: snapshot.DestroyAt, Members: roomMembersDTO(snapshot.Members), PendingRequestCount: snapshot.PendingRequestCount}
}

func roomMembersDTO(members []service.RoomMemberView) []dto.RoomMemberDTO {
	result := make([]dto.RoomMemberDTO, 0, len(members))
	for _, member := range members {
		result = append(result, dto.RoomMemberDTO{UserID: member.UserID, DisplayName: member.DisplayName, Role: member.Role, Status: member.Status, JoinedAt: member.JoinedAt, OnlineStatus: member.OnlineStatus})
	}
	return result
}

func joinRequestDTO(request service.JoinRequestView) dto.JoinRequestDTO {
	return dto.JoinRequestDTO{RequestID: request.RequestID, RoomCode: request.RoomCode, UserID: request.UserID, DisplayName: request.DisplayName, Status: request.Status, CreatedAt: request.CreatedAt, ExpiresAt: request.ExpiresAt}
}
