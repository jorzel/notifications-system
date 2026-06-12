// Package handler implements the generated api.StrictServerInterface,
// translating between the HTTP API types and application use cases.
// Handlers return domain errors as-is; the server's error handler maps
// them to HTTP responses.
package handler

import (
	"context"

	"github.com/jorzel/notifications-system/internal/api"
	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
)

// Handler serves the notification API endpoints.
type Handler struct {
	notifSvc *appnotification.Service
	tmplSvc  *apptemplate.Service
}

var _ api.StrictServerInterface = (*Handler)(nil)

// New creates a Handler.
func New(notifSvc *appnotification.Service, tmplSvc *apptemplate.Service) *Handler {
	return &Handler{
		notifSvc: notifSvc,
		tmplSvc:  tmplSvc,
	}
}

// SendNotification accepts a single notification for asynchronous delivery.
func (h *Handler) SendNotification(ctx context.Context, request api.SendNotificationRequestObject) (api.SendNotificationResponseObject, error) {
	accepted, err := h.send(ctx, request.Body)
	if err != nil {
		return nil, err
	}
	return api.SendNotification202JSONResponse(*accepted), nil
}

// SendNotificationBatch processes items independently and reports a
// result per item, so a batch can partially succeed.
func (h *Handler) SendNotificationBatch(ctx context.Context, request api.SendNotificationBatchRequestObject) (api.SendNotificationBatchResponseObject, error) {
	results := make([]api.BatchItemResult, 0, len(request.Body.Notifications))
	for i := range request.Body.Notifications {
		result := api.BatchItemResult{Index: i}

		accepted, err := h.send(ctx, &request.Body.Notifications[i])
		if err != nil {
			_, errResp := MapError(err)
			result.Error = &errResp
		} else {
			result.Notification = accepted
		}

		results = append(results, result)
	}
	return api.SendNotificationBatch202JSONResponse(api.SendNotificationBatchResponse{Results: results}), nil
}

// ListTemplates returns available templates, optionally filtered by type.
func (h *Handler) ListTemplates(ctx context.Context, request api.ListTemplatesRequestObject) (api.ListTemplatesResponseObject, error) {
	var typeFilter domainnotification.Type
	if request.Params.Type != nil {
		typeFilter = domainnotification.Type(*request.Params.Type)
	}

	templates, err := h.tmplSvc.List(ctx, typeFilter)
	if err != nil {
		return nil, err
	}

	summaries := make([]api.TemplateSummary, 0, len(templates))
	for _, tmpl := range templates {
		summaries = append(summaries, api.TemplateSummary{
			Id:        tmpl.ID,
			Type:      api.NotificationType(tmpl.Type),
			Name:      tmpl.Name,
			IsDefault: tmpl.IsDefault,
		})
	}
	return api.ListTemplates200JSONResponse(api.TemplateList{Templates: summaries}), nil
}

// GetTemplate returns template details. IDs are type-scoped.
func (h *Handler) GetTemplate(ctx context.Context, request api.GetTemplateRequestObject) (api.GetTemplateResponseObject, error) {
	tmpl, err := h.tmplSvc.Resolve(ctx, request.Id, domainnotification.Type(request.Type))
	if err != nil {
		return nil, err
	}

	return api.GetTemplate200JSONResponse(api.Template{
		Id:        tmpl.ID,
		Type:      api.NotificationType(tmpl.Type),
		Name:      tmpl.Name,
		IsDefault: tmpl.IsDefault,
		Subject:   tmpl.Subject,
		Body:      tmpl.Body,
	}), nil
}

func (h *Handler) send(ctx context.Context, req *api.SendNotificationRequest) (*api.NotificationAccepted, error) {
	resp, err := h.notifSvc.Send(ctx, toSendRequest(req))
	if err != nil {
		return nil, err
	}
	return &api.NotificationAccepted{
		Id:        resp.ID,
		Status:    api.Pending,
		CreatedAt: resp.CreatedAt,
	}, nil
}

func toSendRequest(req *api.SendNotificationRequest) *appnotification.SendRequest {
	var priority domainnotification.Priority
	if req.Priority != nil {
		priority = domainnotification.Priority(*req.Priority)
	}

	return &appnotification.SendRequest{
		Type:     domainnotification.Type(req.Type),
		Priority: priority,
		Recipient: domainnotification.Recipient{
			Email:       deref(req.Recipient.Email),
			PhoneNumber: deref(req.Recipient.PhoneNumber),
			DeviceToken: deref(req.Recipient.DeviceToken),
			UserID:      deref(req.Recipient.UserId),
		},
		TemplateID:   deref(req.TemplateId),
		TemplateData: deref(req.TemplateData),
		Metadata:     deref(req.Metadata),
	}
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
