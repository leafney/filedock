package biz

import (
	"fmt"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
)

type ChatBiz struct {
	chat *service.ChatSvc
}

func NewChatBiz(chat *service.ChatSvc) (*ChatBiz, error) {
	if chat == nil {
		return nil, fmt.Errorf("chat service is required")
	}
	return &ChatBiz{chat: chat}, nil
}

func (b *ChatBiz) ListConversations(userID, roomCode string) ([]dto.ChatConversationDTO, error) {
	items, err := b.chat.ListConversations(userID, roomCode)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ChatConversationDTO, 0, len(items))
	for _, item := range items {
		result = append(result, dto.ChatConversationDTO{PeerUserID: item.PeerUserID, PeerDisplayName: item.PeerDisplayName, PeerRole: item.PeerRole, PeerStatus: item.PeerStatus, PeerOnlineStatus: item.PeerOnlineStatus, UnreadCount: item.UnreadCount, LastMessageAt: item.LastMessageAt, LastMessagePreview: item.LastMessagePreview})
	}
	return result, nil
}

func (b *ChatBiz) ListMessages(userID, roomCode, peerUserID string, query service.ChatHistoryQuery) (dto.ChatMessagePageDTO, error) {
	page, err := b.chat.ListMessages(userID, roomCode, peerUserID, query)
	if err != nil {
		return dto.ChatMessagePageDTO{}, err
	}
	return dto.ChatMessagePageDTO{Items: chatMessageDTOs(page.Items), PreviousCursor: page.PreviousCursor, HasMoreBefore: page.HasMoreBefore, NextCursor: page.NextCursor, HasMoreAfter: page.HasMoreAfter, CurrentReadSequence: page.CurrentReadSequence, PeerReadSequence: page.PeerReadSequence}, nil
}

func (b *ChatBiz) Send(userID, roomCode string, request dto.SendChatMessageRequest) (dto.ChatMessageDTO, error) {
	item, err := b.chat.Send(userID, roomCode, request.RecipientUserID, request.ClientMessageID, request.ContentText)
	return chatMessageDTO(item), err
}

func (b *ChatBiz) MarkRead(userID, roomCode, peerUserID string, sequence int64) (dto.ChatReadDTO, error) {
	item, err := b.chat.MarkRead(userID, roomCode, peerUserID, sequence)
	return dto.ChatReadDTO{ConversationID: item.ConversationID, PeerUserID: item.PeerUserID, UserID: item.UserID, LastReadSequence: item.LastReadSequence, LastReadAt: item.LastReadAt}, err
}

func (b *ChatBiz) Recall(userID, roomCode, messageID string) (dto.ChatMessageDTO, error) {
	item, err := b.chat.Recall(userID, roomCode, messageID)
	return chatMessageDTO(item), err
}

func (b *ChatBiz) DeleteMessage(userID, roomCode, messageID string) error {
	return b.chat.DeleteMessage(userID, roomCode, messageID)
}

func (b *ChatBiz) Forward(userID, roomCode, messageID string, request dto.ForwardChatMessageRequest) ([]dto.ChatMessageDTO, error) {
	items, err := b.chat.Forward(userID, roomCode, messageID, request.ClientMessageID, request.RecipientUserIDs)
	return chatMessageDTOs(items), err
}

func (b *ChatBiz) Search(userID, roomCode, peerUserID string, query service.ChatSearchQuery) (dto.ChatSearchPageDTO, error) {
	page, err := b.chat.Search(userID, roomCode, peerUserID, query)
	if err != nil {
		return dto.ChatSearchPageDTO{}, err
	}
	return dto.ChatSearchPageDTO{Items: chatMessageDTOs(page.Items), PreviousCursor: page.PreviousCursor, HasMoreBefore: page.HasMoreBefore}, nil
}

func chatMessageDTOs(items []service.ChatMessageView) []dto.ChatMessageDTO {
	result := make([]dto.ChatMessageDTO, 0, len(items))
	for _, item := range items {
		result = append(result, chatMessageDTO(item))
	}
	return result
}

func chatMessageDTO(item service.ChatMessageView) dto.ChatMessageDTO {
	return dto.ChatMessageDTO{RoomCode: item.RoomCode, ConversationID: item.ConversationID, MessageID: item.MessageID, ClientMessageID: item.ClientMessageID, SenderUserID: item.SenderUserID, RecipientUserID: item.RecipientUserID, SenderDisplayName: item.SenderDisplayName, ContentText: item.ContentText, IsForwarded: item.IsForwarded, Sequence: item.Sequence, CreatedAt: item.CreatedAt, RecalledAt: item.RecalledAt, RecallDeadline: item.RecallDeadline, Read: item.Read, CanCopy: item.CanCopy, CanRecall: item.CanRecall, CanRecallAndEdit: item.CanRecallAndEdit, CanForward: item.CanForward, CanDelete: item.CanDelete}
}
