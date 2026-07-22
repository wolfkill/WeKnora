package repository

import (
	"context"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSessionRepositoryForTest(t *testing.T) (interfaces.SessionRepository, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Session{}))

	return NewSessionRepository(db), db
}

func createSessionForTest(t *testing.T, db *gorm.DB, tenantID uint64, userID string) *types.Session {
	t.Helper()

	session := &types.Session{
		TenantID: tenantID,
		UserID:   userID,
		Title:    userID + " session",
	}
	if userID == "" {
		session.Title = "legacy tenant session"
	}
	require.NoError(t, db.Create(session).Error)

	return session
}

func countActiveSessionsForTest(t *testing.T, db *gorm.DB, id string) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Model(&types.Session{}).Where("id = ?", id).Count(&count).Error)
	return count
}

func sessionIDsForTest(sessions []*types.Session) []string {
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	return ids
}

func TestSessionRepositoryGetAndListHonorUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")
	legacySession := createSessionForTest(t, db, 1, "")
	_ = createSessionForTest(t, db, 2, "bob")

	_, err := repo.Get(ctx, 1, "bob", aliceSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := repo.Get(ctx, 1, "bob", bobSession.ID)
	require.NoError(t, err)
	require.Equal(t, bobSession.ID, got.ID)

	got, err = repo.Get(ctx, 1, "bob", legacySession.ID)
	require.NoError(t, err)
	require.Equal(t, legacySession.ID, got.ID)

	sessions, err := repo.GetByTenantID(ctx, 1, "bob")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{bobSession.ID, legacySession.ID}, sessionIDsForTest(sessions))

	paged, total, err := repo.GetPagedByTenantID(ctx, 1, "bob", &types.Pagination{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.ElementsMatch(t, []string{bobSession.ID, legacySession.ID}, sessionIDsForTest(paged))
}

func TestSessionRepositoryQueryPagedFiltersWebAndIMSessionsByAgentID(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	require.NoError(t, db.Exec(`
		CREATE TABLE im_channel_sessions (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			platform TEXT,
			chat_id TEXT,
			thread_id TEXT,
			user_id TEXT,
			agent_id TEXT,
			im_channel_id TEXT,
			deleted_at DATETIME
		)
	`).Error)

	webSession := &types.Session{
		TenantID: 1,
		UserID:   "alice",
		Title:    "web agent session",
		LastRequestState: &types.SessionLastRequestState{
			AgentID:      "agent-1",
			AgentEnabled: true,
		},
	}
	require.NoError(t, db.Create(webSession).Error)
	imSession := createSessionForTest(t, db, 1, "alice")
	otherAgentSession := &types.Session{
		TenantID: 1,
		UserID:   "alice",
		Title:    "other agent session",
		LastRequestState: &types.SessionLastRequestState{
			AgentID:      "agent-2",
			AgentEnabled: true,
		},
	}
	require.NoError(t, db.Create(otherAgentSession).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO im_channel_sessions
			(id, session_id, platform, chat_id, thread_id, user_id, agent_id, im_channel_id)
		VALUES
			('im-1', ?, 'wechat', 'chat-1', '', 'alice', 'agent-1', 'channel-1')
	`, imSession.ID).Error)

	items, total, err := repo.QueryPaged(ctx, &types.SessionListQuery{
		TenantID: 1,
		UserID:   "alice",
		AgentID:  "agent-1",
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)

	gotIDs := make([]string, 0, len(items))
	for _, item := range items {
		gotIDs = append(gotIDs, item.ID)
	}
	require.ElementsMatch(t, []string{webSession.ID, imSession.ID}, gotIDs)
}

func TestSessionRepositoryUpdateHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")

	rows, err := repo.Update(ctx, &types.Session{
		ID:       aliceSession.ID,
		TenantID: aliceSession.TenantID,
		Title:    "bob update attempt",
	}, "bob")
	require.NoError(t, err)
	require.Zero(t, rows)

	var unchanged types.Session
	require.NoError(t, db.First(&unchanged, "id = ?", aliceSession.ID).Error)
	require.Equal(t, aliceSession.Title, unchanged.Title)

	rows, err = repo.Update(ctx, &types.Session{
		ID:       aliceSession.ID,
		TenantID: aliceSession.TenantID,
		Title:    "alice updated session",
	}, "alice")
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)

	var changed types.Session
	require.NoError(t, db.First(&changed, "id = ?", aliceSession.ID).Error)
	require.Equal(t, "alice updated session", changed.Title)
}

func TestSessionRepositoryDeleteHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")

	rows, err := repo.Delete(ctx, 1, "bob", aliceSession.ID)
	require.NoError(t, err)
	require.Zero(t, rows)
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, aliceSession.ID))

	rows, err = repo.Delete(ctx, 1, "bob", bobSession.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)
	require.Zero(t, countActiveSessionsForTest(t, db, bobSession.ID))
}

func TestSessionRepositoryBatchDeleteHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")
	legacySession := createSessionForTest(t, db, 1, "")

	rows, err := repo.BatchDelete(ctx, 1, "bob", []string{aliceSession.ID, bobSession.ID, legacySession.ID})
	require.NoError(t, err)
	require.EqualValues(t, 2, rows)
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, aliceSession.ID))
	require.Zero(t, countActiveSessionsForTest(t, db, bobSession.ID))
	require.Zero(t, countActiveSessionsForTest(t, db, legacySession.ID))
}

func TestSessionRepositoryDeleteAllHonorsUserScope(t *testing.T) {
	repo, db := newSessionRepositoryForTest(t)
	ctx := context.Background()
	aliceSession := createSessionForTest(t, db, 1, "alice")
	bobSession := createSessionForTest(t, db, 1, "bob")
	legacySession := createSessionForTest(t, db, 1, "")
	otherTenantSession := createSessionForTest(t, db, 2, "bob")

	rows, err := repo.DeleteAllByTenantID(ctx, 1, "bob")
	require.NoError(t, err)
	require.EqualValues(t, 2, rows)
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, aliceSession.ID))
	require.Zero(t, countActiveSessionsForTest(t, db, bobSession.ID))
	require.Zero(t, countActiveSessionsForTest(t, db, legacySession.ID))
	require.EqualValues(t, 1, countActiveSessionsForTest(t, db, otherTenantSession.ID))
}
