package deploy_test

import (
	"testing"
	"time"

	"github.com/adjust/michaelbot/deploy"
	"github.com/adjust/michaelbot/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

/*
   Test objects
*/
type StoreMock struct {
	mock.Mock
}

func (m *StoreMock) GetQueue(key string) deploy.Queue {
	args := m.Called(key)
	return args.Get(0).(deploy.Queue)
}

func (m *StoreMock) SetQueue(key string, q deploy.Queue) {
	m.Called(key, q)
}

func (m *StoreMock) AddToHistory(key string, d deploy.Deploy) {}

/*
   Tests
*/

func TestChannelDeploys_Current(t *testing.T) {
	current := deploy.New(slack.User{ID: "1", Name: "Test User"}, "Test subject")
	current.StartedAt = time.Now().Add(-5 * time.Minute)

	queue := deploy.NewEmptyQueue()
	queue.Add(current)

	store := new(StoreMock)
	store.
		On("GetQueue", "key1").Return(queue, true).
		On("GetQueue", "key2").Return(deploy.NewEmptyQueue(), false)

	repo := deploy.NewChannelDeploys(store)

	if d, ok := repo.Current("key1"); assert.True(t, ok) {
		assert.Equal(t, current, d)
	}

	_, ok := repo.Current("key2")
	assert.False(t, ok)

	store.AssertExpectations(t)
}

func TestChannelDeploys_Finish(t *testing.T) {
	current := deploy.New(slack.User{ID: "1", Name: "Test User"}, "Test subject")
	current.StartedAt = time.Now().Add(-2 * time.Second)

	queue := deploy.NewEmptyQueue()
	queue.Add(current)

	store := new(StoreMock)
	store.
		On("GetQueue", "key1").Return(queue, true).
		On("GetQueue", "key2").Return(deploy.NewEmptyQueue(), false).
		On("SetQueue", "key1", mock.AnythingOfType("deploy.Queue")).Return()

	repo := deploy.NewChannelDeploys(store)

	if d, ok := repo.Finish("key1"); assert.True(t, ok) {
		assert.Equal(t, current.User, d.User)
		assert.Equal(t, current.Subject, d.Subject)
		assert.WithinDuration(t, time.Now(), d.FinishedAt, time.Second)
		assert.False(t, d.Aborted)
	}

	_, ok := repo.Finish("key2")
	assert.False(t, ok)
}

func TestChannelDeploys_Abort(t *testing.T) {
	current := deploy.New(slack.User{ID: "1", Name: "Test User"}, "Test subject")
	current.StartedAt = time.Now().Add(-2 * time.Second)

	queue := deploy.NewEmptyQueue()
	queue.Add(current)

	store := new(StoreMock)
	store.
		On("GetQueue", "key1").Return(queue, true).
		On("GetQueue", "key2").Return(deploy.NewEmptyQueue(), false).
		On("SetQueue", "key1", mock.AnythingOfType("deploy.Queue")).Return()

	repo := deploy.NewChannelDeploys(store)

	if d, ok := repo.Abort("key1", "something went wrong"); assert.True(t, ok) {
		assert.Equal(t, current.User, d.User)
		assert.Equal(t, current.Subject, d.Subject)
		assert.WithinDuration(t, time.Now(), d.FinishedAt, time.Second)
		assert.True(t, d.Aborted)
	}

	_, ok := repo.Abort("key2", "something went wrong")
	assert.False(t, ok)
}
