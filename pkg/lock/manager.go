package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
)

type Manager struct {
	repo    repository.TaskLockRepository
	timeout time.Duration
}

func NewManager(repo repository.TaskLockRepository, timeout time.Duration) *Manager {
	return &Manager{repo: repo, timeout: timeout}
}

// TryAcquire attempts to acquire an exclusive lock for the given task and player.
// Returns true if this player now holds the lock, false if another player holds it.
func (m *Manager) TryAcquire(ctx context.Context, gameID uint64, taskID string, playerID uint64) (bool, error) {
	if err := m.repo.ReleaseExpired(ctx); err != nil {
		return false, fmt.Errorf("lock.TryAcquire: release expired: %w", err)
	}
	expiresAt := time.Now().Add(m.timeout)
	if err := m.repo.Acquire(ctx, gameID, taskID, playerID, expiresAt); err != nil {
		return false, fmt.Errorf("lock.TryAcquire: acquire: %w", err)
	}
	lock, err := m.repo.Get(ctx, gameID, taskID)
	if err != nil {
		return false, fmt.Errorf("lock.TryAcquire: get: %w", err)
	}
	return lock.PlayerID == playerID, nil
}

// Release releases the lock held for the given task.
func (m *Manager) Release(ctx context.Context, gameID uint64, taskID string) error {
	if err := m.repo.Release(ctx, gameID, taskID); err != nil {
		return fmt.Errorf("lock.Release: %w", err)
	}
	return nil
}
