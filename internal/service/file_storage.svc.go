package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const fileCopyBufferSize = 64 * 1024

var ErrStoredSizeMismatch = errors.New("stored file size mismatch")

type StoredFile struct {
	Path         string
	Size         int64
	DetectedMIME string
}

// PrepareUpload creates a fixed-size temporary file that can receive chunks
// at arbitrary offsets. Existing files are accepted only when their size is
// exactly the declared size, which makes process restarts safe.
func (s *FileStorage) PrepareUpload(roomID, storageName string, declaredSize int64) error {
	if s == nil || declaredSize <= 0 || !validInternalName(storageName) {
		return fmt.Errorf("invalid upload preparation")
	}
	tempDir, err := s.ensureInternalDir(s.uploadRoot, roomID)
	if err != nil {
		return err
	}
	tempPath, err := s.safePath(tempDir, storageName+".part")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create upload temporary file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != declaredSize {
		if err := file.Truncate(declaredSize); err != nil {
			return fmt.Errorf("preallocate upload temporary file: %w", err)
		}
	}
	return nil
}

// WritePart writes exactly length bytes at offset without changing the final
// file name. The caller verifies the payload hash before calling this method.
func (s *FileStorage) WritePart(ctx context.Context, roomID, storageName string, offset, length, declaredSize int64, source io.Reader) error {
	if s == nil || source == nil || offset < 0 || length <= 0 || declaredSize <= 0 || offset > declaredSize-length {
		return fmt.Errorf("invalid upload part")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.PrepareUpload(roomID, storageName, declaredSize); err != nil {
		return err
	}
	tempDir, err := s.internalDir(s.uploadRoot, roomID)
	if err != nil {
		return err
	}
	tempPath, err := s.safePath(tempDir, storageName+".part")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(tempPath, os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open upload temporary file: %w", err)
	}
	defer file.Close()
	buffer := make([]byte, fileCopyBufferSize)
	var written int64
	for written < length {
		if err := ctx.Err(); err != nil {
			return err
		}
		want := length - written
		if int64(len(buffer)) > want {
			buffer = buffer[:want]
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			count, writeErr := file.WriteAt(buffer[:read], offset+written)
			if writeErr != nil {
				return fmt.Errorf("write upload part: %w", writeErr)
			}
			if count != read {
				return io.ErrShortWrite
			}
			written += int64(count)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) && written == length {
				break
			}
			return fmt.Errorf("read upload part: %w", readErr)
		}
		if read == 0 {
			return io.ErrNoProgress
		}
	}
	return file.Sync()
}

// FinalizeUpload verifies the preallocated temporary file and atomically moves
// it to the room's permanent storage directory.
func (s *FileStorage) FinalizeUpload(roomID, storageName string, declaredSize int64) (StoredFile, error) {
	if s == nil || declaredSize <= 0 || !validInternalName(storageName) {
		return StoredFile{}, fmt.Errorf("invalid upload finalization")
	}
	tempDir, err := s.internalDir(s.uploadRoot, roomID)
	if err != nil {
		return StoredFile{}, err
	}
	roomDir, err := s.ensureInternalDir(s.roomsRoot, roomID)
	if err != nil {
		return StoredFile{}, err
	}
	tempPath, err := s.safePath(tempDir, storageName+".part")
	if err != nil {
		return StoredFile{}, err
	}
	finalPath, err := s.safePath(roomDir, storageName)
	if err != nil {
		return StoredFile{}, err
	}
	file, err := os.OpenFile(tempPath, os.O_RDONLY, 0)
	if err != nil {
		return StoredFile{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return StoredFile{}, err
	}
	if info.Size() != declaredSize {
		file.Close()
		return StoredFile{}, fmt.Errorf("%w: temporary file size %d, declared %d", ErrStoredSizeMismatch, info.Size(), declaredSize)
	}
	header := make([]byte, 512)
	read, _ := file.ReadAt(header, 0)
	if err := file.Sync(); err != nil {
		file.Close()
		return StoredFile{}, err
	}
	if err := file.Close(); err != nil {
		return StoredFile{}, err
	}
	if _, err := os.Lstat(finalPath); err == nil {
		return StoredFile{}, fmt.Errorf("stored file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return StoredFile{}, err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return StoredFile{}, fmt.Errorf("move uploaded file: %w", err)
	}
	return StoredFile{Path: finalPath, Size: declaredSize, DetectedMIME: http.DetectContentType(header[:read])}, nil
}

func (s *FileStorage) DeleteTemporaryFile(roomID, storageName string) error {
	if s == nil || !validInternalName(storageName) {
		return fmt.Errorf("invalid temporary upload")
	}
	tempDir, err := s.internalDir(s.uploadRoot, roomID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	path, err := s.safePath(tempDir, storageName+".part")
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type FileStorage struct {
	root       string
	roomsRoot  string
	uploadRoot string
}

func NewFileStorage(dataDir string) (*FileStorage, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("file storage data directory is required")
	}
	root, err := filepath.Abs(filepath.Join(dataDir, "files"))
	if err != nil {
		return nil, fmt.Errorf("resolve file storage root: %w", err)
	}
	roomsRoot := filepath.Join(root, "rooms")
	uploadRoot := filepath.Join(root, "uploads")
	for _, directory := range []string{root, roomsRoot, uploadRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create file storage directory: %w", err)
		}
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve file storage symlinks: %w", err)
	}
	return &FileStorage{root: resolved, roomsRoot: filepath.Join(resolved, "rooms"), uploadRoot: filepath.Join(resolved, "uploads")}, nil
}

// Write streams exactly declaredSize bytes to a temporary file and then moves
// it atomically into the room directory. The callback receives stored bytes.
func (s *FileStorage) Write(ctx context.Context, roomID, storageName string, declaredSize int64, source io.Reader, progress func(int64)) (StoredFile, error) {
	if s == nil || source == nil || declaredSize <= 0 {
		return StoredFile{}, fmt.Errorf("invalid file storage write")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	tempDir, err := s.ensureInternalDir(s.uploadRoot, roomID)
	if err != nil {
		return StoredFile{}, err
	}
	roomDir, err := s.ensureInternalDir(s.roomsRoot, roomID)
	if err != nil {
		return StoredFile{}, err
	}
	if !validInternalName(storageName) {
		return StoredFile{}, fmt.Errorf("invalid internal storage name")
	}
	tempPath, err := s.safePath(tempDir, storageName+".part")
	if err != nil {
		return StoredFile{}, err
	}
	finalPath, err := s.safePath(roomDir, storageName)
	if err != nil {
		return StoredFile{}, err
	}
	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return StoredFile{}, fmt.Errorf("create upload temporary file: %w", err)
	}
	keepTemp := false
	defer func() {
		_ = file.Close()
		if !keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	buffer := make([]byte, fileCopyBufferSize)
	header := make([]byte, 0, 512)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return StoredFile{}, err
		}
		remaining := declaredSize + 1 - written
		if remaining <= 0 {
			return StoredFile{}, fmt.Errorf("%w: content exceeds declared size", ErrStoredSizeMismatch)
		}
		chunk := buffer
		if int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		read, readErr := source.Read(chunk)
		if read > 0 {
			if len(header) < 512 {
				needed := 512 - len(header)
				if read < needed {
					needed = read
				}
				header = append(header, chunk[:needed]...)
			}
			count, writeErr := file.Write(chunk[:read])
			written += int64(count)
			if writeErr != nil {
				return StoredFile{}, fmt.Errorf("write upload temporary file: %w", writeErr)
			}
			if count != read {
				return StoredFile{}, io.ErrShortWrite
			}
			if written > declaredSize {
				return StoredFile{}, fmt.Errorf("%w: content exceeds declared size", ErrStoredSizeMismatch)
			}
			if progress != nil {
				progress(written)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return StoredFile{}, fmt.Errorf("read upload content: %w", readErr)
		}
		if read == 0 {
			return StoredFile{}, io.ErrNoProgress
		}
	}
	if written != declaredSize {
		return StoredFile{}, fmt.Errorf("%w: uploaded %d bytes, declared %d", ErrStoredSizeMismatch, written, declaredSize)
	}
	if err := file.Sync(); err != nil {
		return StoredFile{}, fmt.Errorf("sync upload temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return StoredFile{}, fmt.Errorf("close upload temporary file: %w", err)
	}
	if _, err := os.Lstat(finalPath); err == nil {
		return StoredFile{}, fmt.Errorf("stored file already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return StoredFile{}, fmt.Errorf("inspect stored file target: %w", err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return StoredFile{}, fmt.Errorf("move uploaded file: %w", err)
	}
	keepTemp = true
	detected := http.DetectContentType(header)
	return StoredFile{Path: finalPath, Size: written, DetectedMIME: detected}, nil
}

func (s *FileStorage) Open(roomID, storageName string) (*os.File, os.FileInfo, error) {
	roomDir, err := s.internalDir(s.roomsRoot, roomID)
	if err != nil {
		return nil, nil, err
	}
	path, err := s.safePath(roomDir, storageName)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("stored file is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return file, info, nil
}

func (s *FileStorage) DeleteFile(roomID, storageName string) error {
	roomDir, err := s.internalDir(s.roomsRoot, roomID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	path, err := s.safePath(roomDir, storageName)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *FileStorage) DeleteRoom(roomID string) error {
	if !validInternalName(roomID) {
		return fmt.Errorf("invalid internal room id")
	}
	for _, parent := range []string{s.roomsRoot, s.uploadRoot} {
		path, err := s.safePath(parent, roomID)
		if err != nil {
			return err
		}
		if info, err := os.Lstat(path); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("room storage path is a symbolic link")
			}
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *FileStorage) CleanupTemporaryFiles(before time.Time) error {
	return filepath.WalkDir(s.uploadRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("temporary upload is a symbolic link")
		}
		if info.ModTime().Before(before) {
			return os.Remove(path)
		}
		return nil
	})
}

func (s *FileStorage) ensureInternalDir(parent, internalName string) (string, error) {
	path, err := s.safePath(parent, internalName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	return s.verifyDirectory(path)
}

func (s *FileStorage) internalDir(parent, internalName string) (string, error) {
	path, err := s.safePath(parent, internalName)
	if err != nil {
		return "", err
	}
	return s.verifyDirectory(path)
}

func (s *FileStorage) verifyDirectory(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("file storage path is not a directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !withinRoot(s.root, resolved) {
		return "", fmt.Errorf("file storage path escapes root")
	}
	return resolved, nil
}

func (s *FileStorage) safePath(parent string, internalNames ...string) (string, error) {
	for _, name := range internalNames {
		base := strings.TrimSuffix(name, ".part")
		if !validInternalName(base) {
			return "", fmt.Errorf("invalid internal file storage name")
		}
	}
	path := filepath.Join(append([]string{parent}, internalNames...)...)
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !withinRoot(s.root, abs) {
		return "", fmt.Errorf("file storage path escapes root")
	}
	return abs, nil
}

func withinRoot(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func validInternalName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
