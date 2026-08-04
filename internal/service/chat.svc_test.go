package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
)

// newChatFixture creates a real room with three active members. The helper
// deliberately uses the existing room/session services so chat tests exercise
// the same membership and identity invariants as production.
func newChatFixture(t *testing.T) (*ChatSvc, Principal, Principal, Principal, RoomSnapshot, *StreamHub) {
	t.Helper()
	session, db, _ := newSessionTestService(t)
	owner, err := session.Create("甲一", "")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	guest, err := session.Create("乙二", "")
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	third, err := session.Create("丙三", "")
	if err != nil {
		t.Fatalf("create third: %v", err)
	}
	hub := NewStreamHub()
	roomSvc, err := NewRoomSvc(db, hub)
	if err != nil {
		t.Fatalf("new room service: %v", err)
	}
	room, err := roomSvc.Create(owner.Principal.UserID, model.JoinModeOpen, "", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := roomSvc.Join(guest.Principal.UserID, room.RoomCode, true, ""); err != nil {
		t.Fatalf("join guest: %v", err)
	}
	if _, err := roomSvc.Join(third.Principal.UserID, room.RoomCode, true, ""); err != nil {
		t.Fatalf("join third: %v", err)
	}
	chat, err := NewChatSvc(db, hub, NewRateLimiter(), nil)
	if err != nil {
		t.Fatalf("new chat service: %v", err)
	}
	return chat, owner.Principal, guest.Principal, third.Principal, room, hub
}

func TestChatSendNormalizesContentAndIsIdempotent(t *testing.T) {
	chat, owner, guest, _, room, hub := newChatFixture(t)
	ownerEvents, unsubscribeOwner := hub.Subscribe(owner.UserID)
	defer unsubscribeOwner()
	guestEvents, unsubscribeGuest := hub.Subscribe(guest.UserID)
	defer unsubscribeGuest()

	view, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "client-1", "  第一行\r\n第二行  ")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if view.ContentText != "第一行\n第二行" || view.Sequence != 1 || view.SenderDisplayName != owner.DisplayName {
		t.Fatalf("unexpected message view: %+v", view)
	}
	retry, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "client-1", "different")
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if retry.MessageID != view.MessageID || retry.Sequence != view.Sequence {
		t.Fatalf("retry created a different message: %+v", retry)
	}
	var count int64
	if err := chat.db.Model(&model.ChatMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("message count = %d, want 1", count)
	}
	for _, events := range []<-chan *StreamEvent{ownerEvents, guestEvents} {
		select {
		case event := <-events:
			if event.Type != "chat.message_created" {
				t.Fatalf("event type = %q", event.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("chat creation event was not delivered")
		}
	}
}

func TestChatContentBoundariesAndPermissions(t *testing.T) {
	chat, owner, guest, third, room, _ := newChatFixture(t)
	if _, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "blank", " \n\t "); err == nil || errx.Code(err) != errc.ErrChatContent {
		t.Fatalf("blank content error = %v, code=%d", err, errx.Code(err))
	}
	if _, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "too-long", strings.Repeat("界", ChatMessageMaxLength+1)); err == nil || errx.Code(err) != errc.ErrChatContent {
		t.Fatalf("long content error = %v, code=%d", err, errx.Code(err))
	}
	if view, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "unicode", strings.Repeat("🙂", ChatMessageMaxLength)); err != nil || view.ContentText == "" {
		t.Fatalf("unicode boundary send: view=%+v error=%v", view, err)
	}
	if _, err := chat.Send(owner.UserID, room.RoomCode, owner.UserID, "self", "no"); err == nil || errx.Code(err) != errc.ErrChatRecipient {
		t.Fatalf("self recipient error = %v, code=%d", err, errx.Code(err))
	}
	if _, err := chat.Send(owner.UserID, room.RoomCode, "missing", "missing", "no"); err == nil || errx.Code(err) != errc.ErrChatRecipient {
		t.Fatalf("missing recipient error = %v, code=%d", err, errx.Code(err))
	}
	if _, err := chat.ListMessages(third.UserID, room.RoomCode, owner.UserID, ChatHistoryQuery{}); err != nil {
		t.Fatalf("active member should access its own conversation: %v", err)
	}
	// A room owner cannot inspect a conversation between two other members.
	if _, err := chat.ListMessages(owner.UserID, room.RoomCode, third.UserID, ChatHistoryQuery{}); err != nil {
		t.Fatalf("owner may chat with an active member: %v", err)
	}
}

func TestChatHistoryReadDeleteRecallAndSearch(t *testing.T) {
	chat, owner, guest, _, room, _ := newChatFixture(t)
	base := time.Unix(1_000_000, 0)
	chat.now = func() time.Time { return base }
	first, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "first", "中文 %_🙂")
	if err != nil {
		t.Fatalf("first send: %v", err)
	}
	chat.now = func() time.Time { return base.Add(time.Second) }
	second, err := chat.Send(guest.UserID, room.RoomCode, owner.UserID, "second", "reply")
	if err != nil {
		t.Fatalf("second send: %v", err)
	}
	page, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].Sequence != second.Sequence || !page.HasMoreBefore {
		t.Fatalf("latest history page = %+v error=%v", page, err)
	}
	if page.PreviousCursor == nil || *page.PreviousCursor != second.Sequence {
		t.Fatalf("previous cursor = %v", page.PreviousCursor)
	}
	older, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{BeforeSequence: page.PreviousCursor, Limit: 30})
	if err != nil || len(older.Items) != 1 || older.Items[0].MessageID != first.MessageID {
		t.Fatalf("exclusive history page = %+v error=%v", older, err)
	}
	if _, err := chat.MarkRead(owner.UserID, room.RoomCode, guest.UserID, 999); err != nil {
		t.Fatalf("mark read: %v", err)
	}
	read, err := chat.MarkRead(owner.UserID, room.RoomCode, guest.UserID, 1)
	if err != nil || read.LastReadSequence != second.Sequence {
		t.Fatalf("read cursor should not move backwards: %+v error=%v", read, err)
	}
	search, err := chat.Search(owner.UserID, room.RoomCode, guest.UserID, ChatSearchQuery{Query: "%_🙂", Limit: 10})
	if err != nil || len(search.Items) != 1 || search.Items[0].MessageID != first.MessageID {
		t.Fatalf("escaped search = %+v error=%v", search, err)
	}
	chat.now = func() time.Time { return base.Add(120 * time.Second) }
	recalled, err := chat.Recall(owner.UserID, room.RoomCode, first.MessageID)
	if err != nil {
		t.Fatalf("recall at boundary: %v", err)
	}
	if recalled.ContentText != "" || recalled.RecalledAt == nil {
		t.Fatalf("recalled view = %+v", recalled)
	}
	guestPage, err := chat.ListMessages(guest.UserID, room.RoomCode, owner.UserID, ChatHistoryQuery{})
	if err != nil || len(guestPage.Items) != 2 || guestPage.Items[0].ContentText != "" {
		t.Fatalf("recalled history = %+v error=%v", guestPage, err)
	}
	search, err = chat.Search(guest.UserID, room.RoomCode, owner.UserID, ChatSearchQuery{Query: "中文"})
	if err != nil || len(search.Items) != 0 {
		t.Fatalf("recalled message found by search = %+v error=%v", search, err)
	}
	if err := chat.DeleteMessage(owner.UserID, room.RoomCode, first.MessageID); err != nil {
		t.Fatalf("local delete: %v", err)
	}
	if err := chat.DeleteMessage(owner.UserID, room.RoomCode, first.MessageID); err != nil {
		t.Fatalf("idempotent local delete: %v", err)
	}
	ownerPage, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{})
	if err != nil || len(ownerPage.Items) != 1 || ownerPage.Items[0].MessageID != second.MessageID {
		t.Fatalf("owner deleted history = %+v error=%v", ownerPage, err)
	}
	guestPage, err = chat.ListMessages(guest.UserID, room.RoomCode, owner.UserID, ChatHistoryQuery{})
	if err != nil || len(guestPage.Items) != 2 {
		t.Fatalf("peer history changed after local delete = %+v error=%v", guestPage, err)
	}
}

func TestChatHistoryAroundAndAfterPagination(t *testing.T) {
	chat, owner, guest, _, room, _ := newChatFixture(t)
	chat.limiter = nil
	base := time.Unix(3_000_000, 0)
	chat.now = func() time.Time { return base }
	messages := make([]ChatMessageView, 0, 50)
	for index := 1; index <= 50; index++ {
		item, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, fmt.Sprintf("context-%d", index), fmt.Sprintf("消息 %d", index))
		if err != nil {
			t.Fatalf("send message %d: %v", index, err)
		}
		messages = append(messages, item)
	}

	around := int64(25)
	page, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{AroundSequence: &around})
	if err != nil {
		t.Fatalf("load around: %v", err)
	}
	if len(page.Items) != ChatMessageContextSize || page.Items[0].Sequence != 10 || page.Items[15].Sequence != around || page.Items[30].Sequence != 40 {
		t.Fatalf("around page = %+v", page)
	}
	if !page.HasMoreBefore || page.PreviousCursor == nil || *page.PreviousCursor != 10 || !page.HasMoreAfter || page.NextCursor == nil || *page.NextCursor != 40 {
		t.Fatalf("around cursors = %+v", page)
	}

	after := int64(40)
	newer, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{AfterSequence: &after, Limit: 5})
	if err != nil || len(newer.Items) != 5 || newer.Items[0].Sequence != 41 || newer.Items[4].Sequence != 45 || !newer.HasMoreBefore || !newer.HasMoreAfter || newer.NextCursor == nil || *newer.NextCursor != 45 {
		t.Fatalf("newer page = %+v error=%v", newer, err)
	}
	after = 45
	latest, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{AfterSequence: &after})
	if err != nil || len(latest.Items) != 5 || latest.Items[0].Sequence != 46 || latest.Items[4].Sequence != 50 || !latest.HasMoreBefore || latest.HasMoreAfter || latest.NextCursor != nil {
		t.Fatalf("latest after page = %+v error=%v", latest, err)
	}

	before := int64(40)
	if _, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{BeforeSequence: &before, AfterSequence: &after}); err == nil || errx.Code(err) != errc.ErrParams {
		t.Fatalf("mixed direction error = %v", err)
	}

	chat.now = func() time.Time { return base.Add(time.Minute) }
	if _, err := chat.Recall(owner.UserID, room.RoomCode, messages[24].MessageID); err != nil {
		t.Fatalf("recall target: %v", err)
	}
	if _, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{AroundSequence: &around}); err == nil || errx.Code(err) != errc.ErrChatMessageNotFound {
		t.Fatalf("recalled target error = %v", err)
	}

	around = 26
	if err := chat.DeleteMessage(owner.UserID, room.RoomCode, messages[25].MessageID); err != nil {
		t.Fatalf("delete target: %v", err)
	}
	if _, err := chat.ListMessages(owner.UserID, room.RoomCode, guest.UserID, ChatHistoryQuery{AroundSequence: &around}); err == nil || errx.Code(err) != errc.ErrChatMessageNotFound {
		t.Fatalf("deleted target error = %v", err)
	}
}

func TestChatForwardIsAtomicAndIdempotent(t *testing.T) {
	chat, owner, guest, third, room, _ := newChatFixture(t)
	source, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "source", "转发内容")
	if err != nil {
		t.Fatalf("source send: %v", err)
	}
	forwarded, err := chat.Forward(owner.UserID, room.RoomCode, source.MessageID, "forward-1", []string{guest.UserID, third.UserID, guest.UserID})
	if err != nil || len(forwarded) != 2 {
		t.Fatalf("forward = %+v error=%v", forwarded, err)
	}
	for _, item := range forwarded {
		if !item.IsForwarded || item.ContentText != "转发内容" || item.MessageID == source.MessageID || item.SenderUserID != owner.UserID {
			t.Fatalf("forward projection leaked source: %+v", item)
		}
	}
	retry, err := chat.Forward(owner.UserID, room.RoomCode, source.MessageID, "forward-1", []string{third.UserID, guest.UserID})
	if err != nil || len(retry) != 2 {
		t.Fatalf("forward retry = %+v error=%v", retry, err)
	}
	var count int64
	if err := chat.db.Model(&model.ChatMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("count forwarded: %v", err)
	}
	if count != 3 {
		t.Fatalf("forward retry count = %d, want 3", count)
	}
	if _, err := chat.Forward(owner.UserID, room.RoomCode, source.MessageID, "forward-invalid", []string{guest.UserID, "invalid"}); err == nil || errx.Code(err) != errc.ErrChatRecipient {
		t.Fatalf("invalid target error = %v, code=%d", err, errx.Code(err))
	}
	var conversations int64
	if err := chat.db.Model(&model.ChatConversation{}).Count(&conversations).Error; err != nil {
		t.Fatalf("count conversations: %v", err)
	}
	if conversations != 2 {
		t.Fatalf("invalid forward created conversation, count=%d", conversations)
	}
}

func TestChatRecallWindowRejectsAfterExpiry(t *testing.T) {
	chat, owner, guest, _, room, _ := newChatFixture(t)
	base := time.Unix(2_000_000, 0)
	chat.now = func() time.Time { return base }
	message, err := chat.Send(owner.UserID, room.RoomCode, guest.UserID, "recall", "撤回")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	chat.now = func() time.Time { return base.Add(121 * time.Second) }
	if _, err := chat.Recall(owner.UserID, room.RoomCode, message.MessageID); err == nil || errx.Code(err) != errc.ErrChatRecallExpired {
		t.Fatalf("expired recall error = %v, code=%d", err, errx.Code(err))
	}
	if _, err := chat.Recall(guest.UserID, room.RoomCode, message.MessageID); err == nil || errx.Code(err) != errc.ErrChatMessageState {
		t.Fatalf("non-sender recall error = %v, code=%d", err, errx.Code(err))
	}
}
