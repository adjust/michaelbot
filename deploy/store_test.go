package deploy_test

import (
	"time"

	"github.com/adjust/michaelbot/deploy"
	"github.com/adjust/michaelbot/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type StoreSuite struct {
	suite.Suite
	Setup func() (store deploy.Store, teardownFn func(), err error)
}

func (suite *StoreSuite) TestGetSet() {
	store, teardown, err := suite.Setup()
	if teardown != nil {
		defer teardown()
	}
	require.NoError(suite.T(), err)

	// Store a value
	channelDeploy := deploy.Deploy{
		User:        slack.User{ID: "1", Name: "Test User"},
		Subject:     "Deploy subject a/b#1 and c/d#2 for @user1 and @user2",
		StartedAt:   time.Now().Round(0).Add(-5 * time.Minute).UTC(),
		FinishedAt:  time.Now().Round(0).Add(-1 * time.Minute).UTC(),
		Aborted:     true,
		AbortReason: "something went wrong",
		PullRequests: []deploy.PullRequestReference{
			{ID: "1", Repository: "a/b"},
			{ID: "2", Repository: "c/d"},
		},
		Subscribers: []deploy.UserReference{
			{Name: "user1"},
			{Name: "user2"},
		},
	}

	queue := deploy.NewEmptyQueue()
	queue.Add(channelDeploy)

	store.SetQueue("key1", queue)

	q := store.GetQueue("key1")

	assert.Equal(suite.T(), queue, q)
}
