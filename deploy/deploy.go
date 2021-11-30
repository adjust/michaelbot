package deploy

import (
	"time"

	"github.com/adjust/michaelbot/slack"
)

type Queue struct {
	Items []Deploy
}

func NewEmptyQueue() Queue {
	return Queue{make([]Deploy, 0)}
}

func (q *Queue) Add(d Deploy) {
	q.Items = append(q.Items, d)
}

func (q *Queue) Current() (d Deploy, ok bool) {
	if len(q.Items) > 0 {
		return q.Items[0], true
	} else {
		return Deploy{}, false
	}
}

func (q *Queue) Pop() (d Deploy, ok bool) {
	if len(q.Items) > 0 {
		ok = true
		d = q.Items[0]
		q.Items = q.Items[1:len(q.Items)]
	} else {
		ok = false
		d = Deploy{}
	}

	return d, ok
}

func (q *Queue) IsUserInQueue(u slack.User) bool {
	for _, d := range q.Items {
		if d.User.ID == u.ID {
			return true
		}
	}

	return false
}

func (q *Queue) ReplaceHeadWith(d Deploy) {
	q.Items[0] = d
}

type Deploy struct {
	User         slack.User
	Subject      string
	StartedAt    time.Time
	FinishedAt   time.Time
	Aborted      bool
	AbortReason  string
	PullRequests []PullRequestReference
	Subscribers  []UserReference
}

func New(user slack.User, subject string) Deploy {
	return Deploy{
		User:         user,
		Subject:      subject,
		PullRequests: FindPullRequestReferences(subject),
		Subscribers:  FindUserReferences(subject),
	}
}

func (d Deploy) Finished() bool {
	return !d.FinishedAt.IsZero()
}

func (d *Deploy) Start() bool {
	if !d.StartedAt.IsZero() {
		return false
	}

	d.StartedAt = time.Now().UTC()
	return true
}

func (d *Deploy) Finish() {
	if d.Finished() {
		return
	}

	d.FinishedAt = time.Now().UTC()
}

func (d *Deploy) Abort(reason string) {
	if d.Finished() {
		return
	}

	d.Finish()
	d.Aborted, d.AbortReason = true, reason
}

func (d1 Deploy) Equal(d2 Deploy) bool {
	return d1.User == d2.User &&
		d1.Subject == d2.Subject &&
		d1.StartedAt.Equal(d2.StartedAt) &&
		d1.FinishedAt.Equal(d2.FinishedAt)
}
