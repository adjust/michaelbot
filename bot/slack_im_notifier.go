package bot

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/adjust/michaelbot/deploy"
	"github.com/adjust/michaelbot/slack"
)

type SlackIMNotifier struct {
	im             *slack.InstantMessenger
	users          *slack.TeamDirectory
	warningTimeout time.Duration
	timer          *time.Timer
	mutex          sync.Mutex
}

func NewSlackIMNotifier(api *slack.WebAPI, warningTimeout time.Duration) *SlackIMNotifier {
	return &SlackIMNotifier{
		im:             slack.NewInstantMessenger(api),
		users:          slack.NewTeamDirectory(api),
		warningTimeout: warningTimeout,
	}
}

func (notifier *SlackIMNotifier) DeployStarted(_ string, d deploy.Deploy) {
	notifier.mutex.Lock()
	defer notifier.mutex.Unlock()

	// stop current timer if exist
	notifier.stopTimer()

	notifier.timer = time.AfterFunc(notifier.warningTimeout, func() {
		message := slack.Message{
			Text: fmt.Sprintf("Your deploy %q was started %s ago. Are you still deploying?", d.Subject, notifier.warningTimeout),
		}

		err := notifier.im.SendMessage(d.User, message)
		if err != nil {
			log.Printf("failed to send an instant message to %s: %s", d.User.Name, err)
		}
	})
}

func (notifier *SlackIMNotifier) DeployCompleted(_ string, d deploy.Deploy) {
	notifier.mutex.Lock()
	notifier.stopTimer()
	notifier.mutex.Unlock()

	var (
		user slack.User
		err  error
	)

	for _, userRef := range d.Subscribers {
		if userRef.ID != "" {
			user.ID, user.Name = userRef.ID, userRef.Name
		} else {
			user, err = notifier.users.Fetch(userRef.Name)
			if err != nil {
				if _, ok := err.(slack.NoSuchUserError); !ok {
					log.Printf("cannot notify %s about completed deploy of %s: %s", user.Name, d.Subject, err)
				}

				continue
			}
		}

		message := slack.Message{
			Text: fmt.Sprintf("%s just deployed %s", d.User, d.Subject),
		}

		err = notifier.im.SendMessage(user, message)
		if err != nil {
			log.Printf("failed to send an instant message to %s: %s", user.Name, err)
			continue
		}
	}
}

func (notifier *SlackIMNotifier) DeployAborted(_ string, _ deploy.Deploy) {
	notifier.mutex.Lock()
	notifier.stopTimer()
	notifier.mutex.Unlock()
}

func (notifier *SlackIMNotifier) stopTimer() {
	if notifier.timer != nil {
		notifier.timer.Stop()
		notifier.timer = nil
	}
}
