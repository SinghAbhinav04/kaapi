package campaign

import "kaapi/api/internal/platform/apperr"

var (
	ErrBadToken = apperr.New(apperr.CodeNotFound, "That unsubscribe link isn't valid any more. You may already be unsubscribed.")
	ErrBadKind  = apperr.Validation(map[string]string{"kind": "must be FESTIVE or ANNOUNCEMENT (offers are announced from the offer itself)"})
)
