package bot_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adjust/michaelbot/bot"
	"github.com/adjust/michaelbot/deploy"
	"github.com/adjust/michaelbot/slack"
	"github.com/stretchr/testify/assert"
)

func TestSlackIMNotifier_DeployCompleted(t *testing.T) {
	const webAPIToken = "xxxxx-token1"

	d := deploy.Deploy{
		User:    slack.User{ID: "U1", Name: "author"},
		Subject: "Deploy subject",
		Subscribers: []deploy.UserReference{
			{ID: "R1", Name: "recipient1"},
			{Name: "nonExistingRecipient"},
			{Name: "recipient2"},
		},
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var (
		requestNum struct{ UsersList, IMOpen int }
		receivers  []string
	)

	mux.HandleFunc("/users.list", func(w http.ResponseWriter, r *http.Request) {
		requestNum.UsersList++
		assert.Equal(t, webAPIToken, r.FormValue("token"))

		fmt.Fprint(w, `{"ok":true,"members":[{"id":"R1","name":"recipient1"},{"id":"R2","name":"recipient2"},{"id":"R3","name":"recipient3"}]}`)
	})
	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMOpen++
		assert.Equal(t, webAPIToken, r.FormValue("token"))

		if userID := r.FormValue("users"); assert.NotEmpty(t, userID) {
			fmt.Fprintf(w, `{"ok":true,"channel":{"id":"DM%s"}}`, userID)
		} else {
			fmt.Fprint(w, `{"ok":false,"error":"user_not_found"}`)
		}
	})
	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, webAPIToken, r.FormValue("token"))

		if channelID := r.FormValue("channel"); assert.NotEmpty(t, channelID) {
			receivers = append(receivers, channelID)
		}

		if msg := r.FormValue("text"); assert.NotEmpty(t, msg) {
			assert.Contains(t, msg, d.User.String())
			assert.Contains(t, msg, d.Subject)
		}

		fmt.Fprint(w, `{"ok":true}`)
	})

	api := slack.NewWebAPI(webAPIToken, nil)
	api.BaseURL = server.URL

	notifier := bot.NewSlackIMNotifier(api, time.Hour)
	notifier.DeployCompleted("", d)

	assert.Equal(t, 1, requestNum.UsersList) // nonExistingRecipient will not hit the cache
	assert.Equal(t, 2, requestNum.IMOpen)

	if assert.Len(t, receivers, 2) {
		assert.Contains(t, receivers, "DMR1")
		assert.Contains(t, receivers, "DMR2")
	}

	// Retry to check user list caching
	receivers = receivers[:0]
	notifier.DeployCompleted("", d)
	assert.Equal(t, 2, requestNum.UsersList) // +1 request because of nonExistingRecipient
	assert.Equal(t, 2, requestNum.IMOpen)    // no new channels are expected to be open

	if assert.Len(t, receivers, 2) {
		assert.Contains(t, receivers, "DMR1")
		assert.Contains(t, receivers, "DMR2")
	}
}

func TestSlackIMNotifier_DeployStart_Warning(t *testing.T) {
	const webAPIToken = "xxxxx-token1"

	d := deploy.Deploy{
		User:    slack.User{ID: "U1", Name: "author"},
		Subject: "Deploy subject",
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var (
		requestNum struct{ IMPostMessage, IMOpen int }
		receivers  []string
	)

	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMOpen++
		assert.Equal(t, webAPIToken, r.FormValue("token"))

		if userID := r.FormValue("users"); assert.NotEmpty(t, userID) {
			fmt.Fprintf(w, `{"ok":true,"channel":{"id":"DM%s"}}`, userID)
		} else {
			fmt.Fprint(w, `{"ok":false,"error":"user_not_found"}`)
		}
	})

	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMPostMessage++
		assert.Equal(t, webAPIToken, r.FormValue("token"))

		if channelID := r.FormValue("channel"); assert.NotEmpty(t, channelID) {
			receivers = append(receivers, channelID)
		}

		if msg := r.FormValue("text"); assert.NotEmpty(t, msg) {
			assert.Contains(t, msg, `Your deploy "Deploy subject" was started`)
		}

		fmt.Fprint(w, `{"ok":true}`)
	})

	api := slack.NewWebAPI(webAPIToken, nil)
	api.BaseURL = server.URL

	notifier := bot.NewSlackIMNotifier(api, 10*time.Millisecond)
	notifier.DeployStarted("", d)
	time.Sleep(20 * time.Millisecond)

	assert.Equal(t, 1, requestNum.IMOpen)
	assert.Equal(t, 1, requestNum.IMPostMessage)

	if assert.Len(t, receivers, 1) {
		assert.Contains(t, receivers, "DMU1")
	}

	notifier.DeployCompleted("", d)
}

func TestSlackIMNotifier_DeployStart_CompletedBeforeWarning(t *testing.T) {
	const webAPIToken = "xxxxx-token1"

	d := deploy.Deploy{
		User:    slack.User{ID: "U1", Name: "author"},
		Subject: "Deploy subject",
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var (
		requestNum struct{ IMPostMessage, IMOpen int }
	)

	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMOpen++
		fmt.Println(r)
	})

	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMPostMessage++
	})

	api := slack.NewWebAPI(webAPIToken, nil)
	api.BaseURL = server.URL

	notifier := bot.NewSlackIMNotifier(api, 100*time.Millisecond)
	notifier.DeployStarted("", d)
	time.Sleep(10 * time.Millisecond)
	notifier.DeployCompleted("", d)

	assert.Equal(t, 0, requestNum.IMOpen)
	assert.Equal(t, 0, requestNum.IMPostMessage)
}

func TestSlackIMNotifier_DeployStart_AbortBeforeWarning(t *testing.T) {
	const webAPIToken = "xxxxx-token1"

	d := deploy.Deploy{
		User:    slack.User{ID: "U1", Name: "author"},
		Subject: "Deploy subject",
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	var (
		requestNum struct{ IMPostMessage, IMOpen int }
	)

	mux.HandleFunc("/conversations.open", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMOpen++
		fmt.Println(r)
	})

	mux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		requestNum.IMPostMessage++
	})

	api := slack.NewWebAPI(webAPIToken, nil)
	api.BaseURL = server.URL

	notifier := bot.NewSlackIMNotifier(api, 100*time.Millisecond)
	notifier.DeployStarted("", d)
	time.Sleep(10 * time.Millisecond)
	notifier.DeployAborted("", d)

	assert.Equal(t, 0, requestNum.IMOpen)
	assert.Equal(t, 0, requestNum.IMPostMessage)
}
