package media

import (
	"fmt"
	"os"
	"path/filepath"

	tele "gopkg.in/telebot.v3"
)

// Storage abstracts access to media files. The current implementation reads from
// local disk; a future implementation can read from S3 without changing usecase code.
type Storage interface {
	GetFile(name string) (*tele.Document, error)
	GetPhoto(name string) (*tele.Photo, error)
	GetAnimation(name string) (*tele.Animation, error)
}

// LocalStorage reads media files from a directory on the local filesystem.
type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
	return &LocalStorage{basePath: basePath}
}

func (s *LocalStorage) GetFile(name string) (*tele.Document, error) {
	path := s.fullPath(name)
	if err := s.checkExists(path); err != nil {
		return nil, err
	}
	f := tele.FromDisk(path)
	doc := &tele.Document{File: f, FileName: filepath.Base(path)}
	return doc, nil
}

func (s *LocalStorage) GetPhoto(name string) (*tele.Photo, error) {
	path := s.fullPath(name)
	if err := s.checkExists(path); err != nil {
		return nil, err
	}
	f := tele.FromDisk(path)
	photo := &tele.Photo{File: f}
	return photo, nil
}

func (s *LocalStorage) GetAnimation(name string) (*tele.Animation, error) {
	path := s.fullPath(name)
	if err := s.checkExists(path); err != nil {
		return nil, err
	}
	f := tele.FromDisk(path)
	// FileName must be set so that telebot propagates it into the multipart
	// Content-Disposition header. Without it Telegram cannot identify the file
	// type and stores the upload as a Document (shows as a file icon in chat).
	anim := &tele.Animation{File: f, FileName: filepath.Base(path)}
	return anim, nil
}

func (s *LocalStorage) fullPath(name string) string {
	return filepath.Join(s.basePath, name)
}

func (s *LocalStorage) checkExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("media.LocalStorage: file not found: %s", path)
	}
	return nil
}
