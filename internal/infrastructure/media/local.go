package media

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	tele "gopkg.in/telebot.v3"
)

// Storage abstracts access to media files. The current implementation reads from
// local disk; a future implementation can read from S3 without changing usecase code.
//
// CacheFileID lets callers register the Telegram file_id returned by Send for a
// previously uploaded media item. Subsequent Get* calls for the same name will
// then return a media object backed by the cached file_id instead of re-reading
// the file from disk — telebot skips multipart upload when File.FileID is set,
// which dramatically reduces Telegram per-chat flood pressure for hot files
// (notably task_10 meme GIFs).
type Storage interface {
	GetFile(name string) (*tele.Document, error)
	GetPhoto(name string) (*tele.Photo, error)
	GetAnimation(name string) (*tele.Animation, error)
	CacheFileID(name string, fileID string)
}

// LocalStorage reads media files from a directory on the local filesystem.
// Cached file_ids are persisted as JSON to cacheFile so that a bot restart does
// not force every meme/GIF to be re-uploaded — re-uploads are what trip the
// Telegram per-chat 429 flood limit during task_10. If cacheFile is empty,
// caching stays purely in-memory.
type LocalStorage struct {
	basePath  string
	cacheFile string

	mu    sync.Mutex
	cache map[string]string // name → file_id
}

func NewLocalStorage(basePath, cacheFile string) *LocalStorage {
	s := &LocalStorage{
		basePath:  basePath,
		cacheFile: cacheFile,
		cache:     make(map[string]string),
	}
	s.loadCache()
	return s
}

func (s *LocalStorage) GetFile(name string) (*tele.Document, error) {
	if id, ok := s.cachedID(name); ok {
		return &tele.Document{File: tele.File{FileID: id}, FileName: filepath.Base(name)}, nil
	}
	path := s.fullPath(name)
	if err := s.checkExists(path); err != nil {
		return nil, err
	}
	f := tele.FromDisk(path)
	doc := &tele.Document{File: f, FileName: filepath.Base(path)}
	return doc, nil
}

func (s *LocalStorage) GetPhoto(name string) (*tele.Photo, error) {
	if id, ok := s.cachedID(name); ok {
		return &tele.Photo{File: tele.File{FileID: id}}, nil
	}
	path := s.fullPath(name)
	if err := s.checkExists(path); err != nil {
		return nil, err
	}
	f := tele.FromDisk(path)
	photo := &tele.Photo{File: f}
	return photo, nil
}

func (s *LocalStorage) GetAnimation(name string) (*tele.Animation, error) {
	if id, ok := s.cachedID(name); ok {
		// FileName is still set: telebot ignores it for cloud files but it keeps
		// debug output / Telegram metadata consistent across cached/uncached paths.
		return &tele.Animation{File: tele.File{FileID: id}, FileName: filepath.Base(name)}, nil
	}
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

// CacheFileID stores the Telegram file_id returned by a successful Send so that
// subsequent Get* calls for the same name skip the disk upload. If cacheFile
// was configured, the updated map is also written to disk atomically.
func (s *LocalStorage) CacheFileID(name string, fileID string) {
	if fileID == "" {
		return
	}
	s.mu.Lock()
	if s.cache[name] == fileID {
		s.mu.Unlock()
		return
	}
	s.cache[name] = fileID
	snapshot := make(map[string]string, len(s.cache))
	for k, v := range s.cache {
		snapshot[k] = v
	}
	s.mu.Unlock()

	s.persistCache(snapshot)
}

func (s *LocalStorage) cachedID(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.cache[name]
	return id, ok && id != ""
}

// CacheSize returns the number of file_ids currently held in memory.
// Intended for startup logging only; not part of the Storage interface.
func (s *LocalStorage) CacheSize() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cache)
}

// loadCache reads the persisted file_id map from cacheFile, if it exists.
// Any error (missing file, malformed JSON) is non-fatal: we start with an empty
// cache and re-populate it from successful Sends.
func (s *LocalStorage) loadCache() {
	if s.cacheFile == "" {
		return
	}
	data, err := os.ReadFile(s.cacheFile)
	if err != nil {
		// Missing/unreadable cache file: cold start, nothing to do.
		return
	}
	var loaded map[string]string
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	s.mu.Lock()
	for k, v := range loaded {
		if v != "" {
			s.cache[k] = v
		}
	}
	s.mu.Unlock()
}

// persistCache writes snapshot to cacheFile atomically (tmp + rename) so a crash
// or concurrent write cannot leave a partially-written JSON.
func (s *LocalStorage) persistCache(snapshot map[string]string) {
	if s.cacheFile == "" {
		return
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	if dir := filepath.Dir(s.cacheFile); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.cacheFile), ".media_cache-*.json")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, s.cacheFile); err != nil {
		_ = os.Remove(tmpName)
	}
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
