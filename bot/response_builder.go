package bot

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/adjust/michaelbot/deploy"
	"github.com/adjust/michaelbot/github"
	"github.com/adjust/michaelbot/slack"
)

const (
	helpMessage = `Available commands:

/deploy help — print help (this message)
/deploy <subject> — announce deploy of <subject> in channel
/deploy status — show deploy status in channel
/deploy done — finish deploy
/deploy abort [<reason>] — abort current deploy, optionally providing a reason
/deploy history — get a link to history of deploys in this channel`
	errorMessage                   = "`%s` returned an error %s"
	noRunningDeploysMessage        = "No one is deploying at the moment"
	singleDeployStatusMessage      = "%s is deploying %s since %s. There are no other deploys scheduled yet."
	deployQueueStatusMessage       = "%s is deploying %s since %s. The queue:\n %s"
	alreadyInQueueMessage          = "%s is already in queue"
	deployConflictMessage          = "%s is deploying since %s, your PR has been added to the queue. You can type `/deploy done` if you think the current deploy is finished or type `/deploy status` to print the queue."
	deployDoneMessage              = "%s done deploying"
	deployInterruptedMessage       = "%s has finished the deploy started by %s"
	deployAnnouncementMessage      = "%s is about to deploy %s"
	deployHistoryLinkMessage       = "Click <https://%s/%s|here> to see deploy history in this channel"
	deployAbortedMessage           = "%s has aborted the deploy"
	deployAbortedWithReasonMessage = "%s has aborted the deploy (%s)"
	userLeftQueueMessage           = "Your scheduled deploy has been cancelled"
	userIsNotInQueueMessage        = "You are not in the queue"
)

type ResponseBuilder struct {
	githubClient *github.Client
}

func NewResponseBuilder(githubClient *github.Client) *ResponseBuilder {
	return &ResponseBuilder{githubClient: githubClient}
}

func (b *ResponseBuilder) HelpMessage() *slack.Response {
	return newUserMessage(slack.EscapeMessage(helpMessage))
}

func (b *ResponseBuilder) ErrorMessage(cmd string, err error) *slack.Response {
	return newUserMessage(fmt.Sprintf(errorMessage, cmd, err))
}

func (b *ResponseBuilder) NoRunningDeploysMessage() *slack.Response {
	return newUserMessage(slack.EscapeMessage(noRunningDeploysMessage))
}

func (b *ResponseBuilder) UserLeftTheQueueMessage() *slack.Response {
	return newUserMessage(slack.EscapeMessage(userLeftQueueMessage))
}

func (b *ResponseBuilder) NotInTheQueueMessage() *slack.Response {
	return newUserMessage(slack.EscapeMessage(userIsNotInQueueMessage))
}

func (b *ResponseBuilder) DeployStatusMessage(deploys []deploy.Deploy) *slack.Response {
	if len(deploys) == 1 {
		d := deploys[0]

		return newUserMessage(fmt.Sprintf(singleDeployStatusMessage, d.User, slack.EscapeMessage(d.Subject), d.StartedAt.Format(time.RFC822)))
	} else {
		current := deploys[0]
		rest := deploys[1:]
		users := make([]string, len(rest))

		for i, d := range rest {
			users[i] = fmt.Sprintf("%d. %s [%s]", i+1, d.User, d.Subject)
		}

		return newUserMessage(
			fmt.Sprintf(deployQueueStatusMessage, current.User, slack.EscapeMessage(current.Subject), current.StartedAt.Format(time.RFC822), strings.Join(users, "\n")),
		)
	}
}

func (b *ResponseBuilder) DeployInProgressMessage(d deploy.Deploy) *slack.Response {
	return newUserMessage(fmt.Sprintf(deployConflictMessage, d.User, d.StartedAt.Format(time.RFC822)))
}

func (b *ResponseBuilder) UserIsInQeueueMessage(u slack.User) *slack.Response {
	return newUserMessage(fmt.Sprintf(alreadyInQueueMessage, u))
}

func (b *ResponseBuilder) DeployInterruptedAnnouncement(d deploy.Deploy, user slack.User) *slack.Response {
	return newAnnouncement(fmt.Sprintf(deployInterruptedMessage, user, d.User))
}

func (b *ResponseBuilder) DeployAnnouncement(d deploy.Deploy) *slack.Response {
	responseText := fmt.Sprintf(deployAnnouncementMessage, d.User, d.Subject)
	response := newAnnouncement(responseText)
	for _, ref := range d.PullRequests {
		pr, err := b.githubClient.GetPullRequest(ref.Repository, ref.ID)
		if err != nil {
			response.Attachments = append(response.Attachments, slack.Attachment{
				Title:     ref.Repository + "#" + ref.ID,
				TitleLink: "https://github.com/" + ref.Repository + "/pulls/" + ref.ID,
			})
			continue
		}

		response.Attachments = append(response.Attachments, slack.Attachment{
			AuthorName: pr.Author.Name,
			Title:      fmt.Sprintf("PR #%d: %s", pr.Number, slack.EscapeMessage(pr.Title)),
			TitleLink:  pr.URL,
			Text:       pr.Body,
			Markdown:   true,
		})
	}

	return response
}

func (b *ResponseBuilder) DeployDoneAnnouncement(user slack.User) *slack.Response {
	return newAnnouncement(fmt.Sprintf(deployDoneMessage, user))
}

func (b *ResponseBuilder) DeployAbortedAnnouncement(reason string, user slack.User) *slack.Response {
	if reason != "" {
		return newAnnouncement(fmt.Sprintf(deployAbortedWithReasonMessage, user, reason))
	} else {
		return newAnnouncement(fmt.Sprintf(deployAbortedMessage, user))
	}
}

func (*ResponseBuilder) DeployHistoryLink(host, channelID, authToken string) *slack.Response {
	host = strings.TrimSuffix(strings.TrimSuffix(host, ":80"), ":443")
	path := &url.URL{Path: channelID}

	if authToken != "" {
		q := path.Query()
		q.Set("token", authToken)
		path.RawQuery = q.Encode()
	}

	return newUserMessage(fmt.Sprintf(deployHistoryLinkMessage, host, path))
}

func newUserMessage(s string) *slack.Response {
	return slack.NewEphemeralResponse(s)
}

func newAnnouncement(s string) *slack.Response {
	return slack.NewInChannelResponse(s)
}
