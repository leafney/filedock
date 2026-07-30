package biz

import (
	"fmt"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
)

type SessionBiz struct {
	session  *service.SessionSvc
	nickname *service.NicknameSvc
}

func NewSessionBiz(session *service.SessionSvc, nickname *service.NicknameSvc) (*SessionBiz, error) {
	if session == nil {
		return nil, fmt.Errorf("session service is required")
	}
	if nickname == nil {
		return nil, fmt.Errorf("nickname service is required")
	}
	return &SessionBiz{session: session, nickname: nickname}, nil
}

func (b *SessionBiz) Create(request dto.CreateSessionRequest) (service.SessionResult, error) {
	return b.session.Create(request.DisplayName, request.DeviceHint)
}

func (b *SessionBiz) View(principal service.Principal) dto.SessionDTO {
	return dto.SessionDTO{UserID: principal.UserID, DisplayName: principal.DisplayName, SessionExpiresAt: principal.SessionExpiresAt}
}

func (b *SessionBiz) UpdateName(userID string, request dto.UpdateSessionRequest) (service.Principal, error) {
	return b.session.UpdateName(userID, request.DisplayName)
}

func (b *SessionBiz) Reset(userID string) error {
	return b.session.Reset(userID)
}

func (b *SessionBiz) RandomNickname() (dto.RandomNicknameDTO, error) {
	name, err := b.nickname.Random()
	if err != nil {
		return dto.RandomNicknameDTO{}, err
	}
	return dto.RandomNicknameDTO{DisplayName: name}, nil
}
