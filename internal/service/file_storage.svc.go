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
