package lock_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testGameID   uint64 = 1
	testPlayerID uint64 = 10
	testTaskID          = "task_02"
	testTimeout         = 15 * time.Minute
)

func TestTryAcquire_LockFree_ReturnsTrue(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockTaskLockRepository(ctrl)
	mgr := lock.NewManager(repo, testTimeout)
	ctx := context.Background()

	repo.EXPECT().ReleaseExpired(ctx).Return(nil)
	repo.EXPECT().Acquire(ctx, testGameID, testTaskID, testPlayerID, gomock.Any()).Return(nil)
	repo.EXPECT().Get(ctx, testGameID, testTaskID).Return(&entity.TaskLock{
		PlayerID: testPlayerID,
	}, nil)

	ok, err := mgr.TryAcquire(ctx, testGameID, testTaskID, testPlayerID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestTryAcquire_LockHeldByOtherPlayer_ReturnsFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockTaskLockRepository(ctrl)
	mgr := lock.NewManager(repo, testTimeout)
	ctx := context.Background()

	otherPlayerID := uint64(99)
	repo.EXPECT().ReleaseExpired(ctx).Return(nil)
	repo.EXPECT().Acquire(ctx, testGameID, testTaskID, testPlayerID, gomock.Any()).Return(nil)
	repo.EXPECT().Get(ctx, testGameID, testTaskID).Return(&entity.TaskLock{
		PlayerID: otherPlayerID,
	}, nil)

	ok, err := mgr.TryAcquire(ctx, testGameID, testTaskID, testPlayerID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTryAcquire_ReleaseExpiredCalledFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockTaskLockRepository(ctrl)
	mgr := lock.NewManager(repo, testTimeout)
	ctx := context.Background()

	// gomock enforces order via InOrder
	gomock.InOrder(
		repo.EXPECT().ReleaseExpired(ctx).Return(nil),
		repo.EXPECT().Acquire(ctx, testGameID, testTaskID, testPlayerID, gomock.Any()).Return(nil),
		repo.EXPECT().Get(ctx, testGameID, testTaskID).Return(&entity.TaskLock{PlayerID: testPlayerID}, nil),
	)

	ok, err := mgr.TryAcquire(ctx, testGameID, testTaskID, testPlayerID)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRelease_CallsRepoRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockTaskLockRepository(ctrl)
	mgr := lock.NewManager(repo, testTimeout)
	ctx := context.Background()

	repo.EXPECT().Release(ctx, testGameID, testTaskID).Return(nil)

	err := mgr.Release(ctx, testGameID, testTaskID)
	require.NoError(t, err)
}
