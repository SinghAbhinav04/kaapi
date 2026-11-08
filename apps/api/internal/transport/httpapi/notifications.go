package httpapi

import (
	"net/http"

	"kaapi/api/internal/auth"
	"kaapi/api/internal/notify"
	"kaapi/api/internal/platform/httpx"
)

// viewer is the signed-in staff member as alerts see them; guests have no alerts.
func viewer(r *http.Request) (notify.Viewer, error) {
	p := principal(r.Context())
	if !p.Role.IsStaff() {
		return notify.Viewer{}, auth.ErrForbidden
	}
	return notify.Viewer{User: p.UserID, Team: p.Role}, nil
}

// listNotifications: GET /v1/staff/notifications, the newest alerts for the caller's team.
func (a *API) listNotifications(w http.ResponseWriter, r *http.Request) error {
	v, err := viewer(r)
	if err != nil {
		return err
	}
	p := httpx.NewParams(r)
	limit := p.Int("limit", notify.DefaultLimit, 1, notify.MaxLimit)
	if err := p.Err(); err != nil {
		return err
	}
	page, err := a.notify.List(r.Context(), v, limit)
	if err != nil {
		return err
	}
	out := NotificationsResponse{Notifications: make([]NotificationDTO, len(page.Items)), Unread: page.Unread}
	for i, n := range page.Items {
		out.Notifications[i] = NotificationDTO{ID: n.ID, Type: n.Type, Body: n.Body, EnquiryRef: n.EnquiryRef, BookingRef: n.BookingRef,
			CreatedAt: n.CreatedAt, ReadAt: n.ReadAt}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// readNotification: POST /v1/staff/notifications/{id}/read.
func (a *API) readNotification(w http.ResponseWriter, r *http.Request) error {
	v, err := viewer(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, notify.ErrNotFound)
	if err != nil {
		return err
	}
	if err := a.notify.MarkRead(r.Context(), v, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// readAllNotifications: POST /v1/staff/notifications/read marks every visible alert read.
func (a *API) readAllNotifications(w http.ResponseWriter, r *http.Request) error {
	v, err := viewer(r)
	if err != nil {
		return err
	}
	if err := a.notify.MarkAllRead(r.Context(), v); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
