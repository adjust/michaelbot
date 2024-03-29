package bot

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/adjust/michaelbot/auth"
	"github.com/adjust/michaelbot/deploy"
	"github.com/adjust/michaelbot/github"
	"github.com/adjust/michaelbot/slack"
)

type DeployEventHandler interface {
	DeployStarted(channelID string, d deploy.Deploy)
	DeployCompleted(channelID string, d deploy.Deploy)
	DeployAborted(channelID string, d deploy.Deploy)
}

type Bot struct {
	slackToken    string
	deploys       *deploy.ChannelDeploys
	responses     *ResponseBuilder
	dashboardAuth auth.TokenIssuer

	deployEventHandlers []DeployEventHandler
}

func New(slackToken, githubToken string, store deploy.Store) *Bot {
	return &Bot{
		slackToken:    slackToken,
		deploys:       deploy.NewChannelDeploys(store),
		responses:     NewResponseBuilder(github.NewClient(githubToken, nil)),
		dashboardAuth: auth.None,
	}
}

func (b *Bot) AddDeployEventHandler(h DeployEventHandler) {
	b.deployEventHandlers = append(b.deployEventHandlers, h)
}

func (b *Bot) SetDashboardAuth(issuer auth.TokenIssuer) {
	if issuer == nil {
		b.dashboardAuth = auth.None
	}

	b.dashboardAuth = issuer
}

func (b *Bot) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST requests are supported", http.StatusBadRequest)
		return
	}

	if r.PostFormValue("token") != b.slackToken {
		http.Error(w, "Invalid token", http.StatusForbidden)
		return
	}

	if cmd := r.PostFormValue("command"); cmd != "/deploy" {
		sendImmediateResponse(w, b.responses.ErrorMessage(cmd, errors.New("not supported")))
		return
	}

	channelID := r.PostFormValue("channel_id")
	user := slack.User{
		ID:   r.PostFormValue("user_id"),
		Name: r.PostFormValue("user_name"),
	}

	// TODO: make commands case-insensitive
	subject := strings.TrimSpace(r.PostFormValue("text"))

	switch {
	case subject == "help" || subject == "":
		sendImmediateResponse(w, b.responses.HelpMessage())
	case subject == "status":
		deploys := b.deploys.All(channelID)

		if len(deploys) == 0 {
			sendImmediateResponse(w, b.responses.NoRunningDeploysMessage())
			return
		}

		sendImmediateResponse(w, b.responses.DeployStatusMessage(deploys))
	case subject == "done":
		d, ok := b.deploys.Finish(channelID)

		if !ok {
			sendImmediateResponse(w, b.responses.NoRunningDeploysMessage())
			return
		}

		if d.User.ID == user.ID {
			go sendDelayedResponse(w, r, b.responses.DeployDoneAnnouncement(user))
		} else {
			go sendDelayedResponse(w, r, b.responses.DeployInterruptedAnnouncement(d, user))
		}

		nextDeploy, nextDeployStarted := b.deploys.Current(channelID)
		if nextDeployStarted {
			go sendDelayedResponse(w, r, b.responses.DeployAnnouncement(nextDeploy))

			for _, h := range b.deployEventHandlers {
				go h.DeployStarted(channelID, nextDeploy)
			}
		} else {
			for _, h := range b.deployEventHandlers {
				go h.DeployCompleted(channelID, d)
			}
		}
	case subject == "abort" || strings.HasPrefix(subject, "abort "):
		var reason string
		if strings.HasPrefix(subject, "abort ") && len(subject) > len("abort ") {
			reason = subject[len("abort "):]
		}

		d, ok := b.deploys.Current(channelID)
		if !ok {
			sendImmediateResponse(w, b.responses.NoRunningDeploysMessage())
			return
		}

		if d.User.ID == user.ID {
			d, _ := b.deploys.Abort(channelID, reason)

			go sendDelayedResponse(w, r, b.responses.DeployAbortedAnnouncement(reason, user))

			nextDeploy, nextDeployStarted := b.deploys.Current(channelID)
			if nextDeployStarted {
				go sendDelayedResponse(w, r, b.responses.DeployAnnouncement(nextDeploy))

				for _, h := range b.deployEventHandlers {
					go h.DeployStarted(channelID, nextDeploy)
				}
			} else {
				for _, h := range b.deployEventHandlers {
					go h.DeployAborted(channelID, d)
				}
			}

		} else {
			userLeftQueue := b.deploys.LeaveQueue(channelID, user)
			if userLeftQueue {
				sendImmediateResponse(w, b.responses.UserLeftTheQueueMessage())
			} else {
				sendImmediateResponse(w, b.responses.NotInTheQueueMessage())
			}
		}
	case subject == "history":
		dashboardToken, err := b.dashboardAuth.IssueToken(auth.DefaultTokenLength)
		if err != nil {
			sendImmediateResponse(w, b.responses.ErrorMessage("history", err))
			return
		}

		sendImmediateResponse(w, b.responses.DeployHistoryLink(r.Host, channelID, dashboardToken))
	default:
		d, err := b.deploys.Start(channelID, deploy.New(user, slack.EscapeMessage(subject)))
		if errors.Is(err, deploy.DeployInProgressError) {
			sendImmediateResponse(w, b.responses.DeployInProgressMessage(d))
			return
		} else if errors.Is(err, deploy.AlreadyInQueueError) {
			sendImmediateResponse(w, b.responses.UserIsInQeueueMessage(d.User))
			return
		} else if err != nil {
			log.Printf("failed to start a deploy: (%s)", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Write(nil)

		go sendDelayedResponse(w, r, b.responses.DeployAnnouncement(d))
		for _, h := range b.deployEventHandlers {
			go h.DeployStarted(channelID, d)
		}
	}
}

func sendImmediateResponse(w http.ResponseWriter, response *slack.Response) {
	body, err := json.Marshal(response)
	if err != nil {
		log.Printf("failed to respond to user with %q (%s)", response.Text, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func sendDelayedResponse(w http.ResponseWriter, req *http.Request, response *slack.Response) {
	responseURL := req.PostFormValue("response_url")
	if responseURL == "" {
		log.Printf("cannot send delayed response to a without without response_url")
		return
	}

	body, err := json.Marshal(response)
	if err != nil {
		log.Printf("failed to respond in channel with %s (%s)", response.Text, err)
		return
	}

	slackResponse, err := http.Post(responseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("failed to sent in_channel response (%s)", err)
		return
	}
	slackResponse.Body.Close()
}
