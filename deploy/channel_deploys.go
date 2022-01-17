package deploy

import (
	"errors"

	"github.com/adjust/michaelbot/slack"
)

var (
	AlreadyInQueueError   = errors.New("User is already in queue")
	DeployInProgressError = errors.New("Another deploy is in progress")
)

type ChannelDeploys struct {
	store Store
}

func NewChannelDeploys(store Store) *ChannelDeploys {
	return &ChannelDeploys{store: store}
}

func (repo *ChannelDeploys) All(channelID string) []Deploy {
	queue := repo.store.GetQueue(channelID)

	return queue.Items
}

func (repo *ChannelDeploys) Current(channelID string) (Deploy, bool) {
	queue := repo.store.GetQueue(channelID)

	return queue.Current()
}

func (repo *ChannelDeploys) Start(channelID string, deploy Deploy) (Deploy, error) {
	queue := repo.store.GetQueue(channelID)

	current, deployInProgress := queue.Current()

	if queue.IsUserInQueue(deploy.User) {
		return deploy, AlreadyInQueueError
	}

	if deployInProgress {
		queue.Add(deploy)
		repo.store.SetQueue(channelID, queue)

		return current, DeployInProgressError
	}

	deploy.Start()
	queue.Add(deploy)
	repo.store.SetQueue(channelID, queue)

	return deploy, nil
}

func (repo *ChannelDeploys) Finish(channelID string) (Deploy, bool) {
	queue := repo.store.GetQueue(channelID)
	current, deployInProgress := queue.Pop()

	if !deployInProgress {
		return current, false
	}

	current.Finish()
	repo.store.AddToHistory(channelID, current)

	next, queueIsNotEmpty := queue.Current()

	if queueIsNotEmpty {
		next.Start()
		queue.ReplaceHeadWith(next)
	}

	repo.store.SetQueue(channelID, queue)

	return current, true
}

func (repo *ChannelDeploys) Abort(channelID, reason string) (Deploy, bool) {
	queue := repo.store.GetQueue(channelID)
	current, deployInProgress := queue.Pop()

	if !deployInProgress {
		return current, false
	}

	current.Abort(reason)
	repo.store.AddToHistory(channelID, current)

	next, queueIsNotEmpty := queue.Current()

	if queueIsNotEmpty {
		next.Start()
		queue.ReplaceHeadWith(next)
	}

	repo.store.SetQueue(channelID, queue)

	return current, true
}

func (repo *ChannelDeploys) LeaveQueue(channelID string, user slack.User) bool {
	queue := repo.store.GetQueue(channelID)

	userHasBeenRemoved := queue.RemoveUser(user)

	if userHasBeenRemoved {
		repo.store.SetQueue(channelID, queue)
	}

	return userHasBeenRemoved
}
