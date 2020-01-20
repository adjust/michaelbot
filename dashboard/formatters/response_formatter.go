package formatters

import (
	"net/http"

	"github.com/adjust/michaelbot/deploy"
)

type ResponseFormatter interface {
	RespondWithHistory(http.ResponseWriter, []deploy.Deploy) error
	RespondWithError(http.ResponseWriter, error, int) error
}
