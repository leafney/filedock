package service

import (
	"github.com/leafney/filedock/internal/model"
	"gorm.io/gorm"
)

func listValidTrashNotificationRows(db *gorm.DB, userID string, now int64) ([]fileNotificationRow, error) {
	const query = `
SELECT
    record.id AS record_id,
    record.type,
    record.room_id,
    room.code AS room_code,
    room.title AS room_title,
    record.counterpart_user_id,
    counterpart.display_name AS counterpart_name,
    CASE
      WHEN file.scope = ?
        OR file.uploader_user_id = record.user_id
        OR EXISTS (
          SELECT 1 FROM file_recipients AS visible_recipient
          WHERE visible_recipient.file_id = file.id
            AND visible_recipient.recipient_user_id = record.user_id
        )
      THEN file.original_name
      ELSE file.private_code
    END AS original_name,
    CASE
      WHEN record.type = ? THEN restore_request.rejection_reason
      ELSE trash_cycle.delete_reason
    END AS reason,
    record.restore_request_id AS request_id,
    CASE
      WHEN record.type = ? THEN restore_request.created_at * 1000
      ELSE record.occurred_at_ms
    END AS occurred_at_ms
FROM notification_records AS record
JOIN rooms AS room
  ON room.id = record.room_id
 AND room.status = ?
 AND room.expires_at > ?
JOIN room_members AS current_member
  ON current_member.room_id = record.room_id
 AND current_member.user_id = record.user_id
 AND current_member.status = ?
JOIN room_files AS file
  ON file.id = record.file_id
 AND file.room_id = record.room_id
 AND file.trash_version = record.trash_version
JOIN file_trash_cycles AS trash_cycle
  ON trash_cycle.id = record.trash_cycle_id
 AND trash_cycle.file_id = file.id
 AND trash_cycle.version = record.trash_version
LEFT JOIN file_restore_requests AS restore_request
  ON restore_request.id = record.restore_request_id
 AND restore_request.trash_cycle_id = trash_cycle.id
 AND restore_request.trash_version = record.trash_version
JOIN room_members AS counterpart
  ON counterpart.room_id = record.room_id
 AND counterpart.user_id = record.counterpart_user_id
WHERE record.user_id = ?
  AND record.read_at_ms IS NULL
  AND (
      (record.type = ?
       AND record.user_id = file.uploader_user_id
       AND file.status = ?
       AND trash_cycle.outcome = ?
       AND trash_cycle.deleted_by_user_id = room.owner_user_id
       AND record.counterpart_user_id = room.owner_user_id
       AND file.uploader_user_id <> room.owner_user_id)
   OR (record.type = ?
       AND record.user_id = room.owner_user_id
       AND file.status = ?
       AND trash_cycle.outcome = ?
       AND restore_request.status = ?
       AND restore_request.requester_user_id = record.counterpart_user_id
       AND EXISTS (
         SELECT 1 FROM room_members AS requester_member
         WHERE requester_member.room_id = record.room_id
           AND requester_member.user_id = restore_request.requester_user_id
           AND requester_member.status = ?
       ))
   OR (record.type = ?
       AND record.user_id = file.uploader_user_id
       AND file.status = ?
       AND trash_cycle.outcome = ?
       AND trash_cycle.resolved_by_user_id = room.owner_user_id
       AND record.counterpart_user_id = room.owner_user_id
       AND file.uploader_user_id <> room.owner_user_id)
   OR (record.type = ?
       AND record.user_id = file.uploader_user_id
       AND file.status = ?
       AND trash_cycle.outcome = ?
       AND restore_request.status = ?
       AND restore_request.requester_user_id = record.user_id
       AND restore_request.decided_by_user_id = room.owner_user_id
       AND record.counterpart_user_id = room.owner_user_id)
   OR (record.type = ?
       AND record.user_id = file.uploader_user_id
       AND file.status = ?
       AND trash_cycle.outcome = ?
       AND trash_cycle.resolved_by_user_id = room.owner_user_id
       AND record.counterpart_user_id = room.owner_user_id
       AND file.uploader_user_id <> room.owner_user_id)
  )`
	var rows []fileNotificationRow
	err := db.Raw(query,
		model.FileScopeShared,
		NotificationTypeFileRestoreRejected,
		NotificationTypeFileRestoreRequested,
		model.RoomStatusActive,
		now,
		model.MemberStatusActive,
		userID,
		NotificationTypeFileTrashedByOwner,
		model.FileStatusTrashed,
		model.FileTrashActive,
		NotificationTypeFileRestoreRequested,
		model.FileStatusTrashed,
		model.FileTrashActive,
		model.FileRestorePending,
		model.MemberStatusActive,
		NotificationTypeFileRestoredByOwner,
		model.FileStatusAvailable,
		model.FileTrashRestored,
		NotificationTypeFileRestoreRejected,
		model.FileStatusTrashed,
		model.FileTrashActive,
		model.FileRestoreRejected,
		NotificationTypeFilePurgedByOwner,
		model.FileStatusPurged,
		model.FileTrashPurged,
	).Scan(&rows).Error
	return rows, err
}
