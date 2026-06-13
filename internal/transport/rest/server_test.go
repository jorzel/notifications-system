package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appnotification "github.com/jorzel/notifications-system/internal/app/notification"
	apptemplate "github.com/jorzel/notifications-system/internal/app/template"
	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/notification/mocks"
	infratemplate "github.com/jorzel/notifications-system/internal/infrastructure/template"
	"github.com/jorzel/notifications-system/internal/transport/rest/api"
)

const welcomeEmailTemplate = `---
name: Welcome Email
subject: "Welcome, {{.name}}!"
---
<h1>Hello, {{.name}}!</h1>
`

func newTestServer(t *testing.T, publisher domainnotification.Publisher) *echo.Echo {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{
		"email/welcome.html": welcomeEmailTemplate,
		"email/default.html": "---\nsubject: \"{{.subject}}\"\n---\n<p>{{.message}}</p>",
		"sms/default.txt":    "{{.message}}",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	repo, err := infratemplate.NewFileRepository(dir)
	require.NoError(t, err)
	tmplSvc := apptemplate.NewService(repo)
	notifSvc := appnotification.NewService(publisher, tmplSvc)

	server, err := NewServer(notifSvc, tmplSvc, zerolog.Nop())
	require.NoError(t, err)
	return server
}

func doRequest(t *testing.T, server *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	return rec
}

func TestSendNotificationAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)

	var published *domainnotification.Message
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, msg *domainnotification.Message) error {
			published = msg
			return nil
		})

	server := newTestServer(t, publisher)

	rec := doRequest(t, server, http.MethodPost, "/api/v1/notifications", `{
		"type": "email",
		"recipient": {"email": "anna.kowalska@example.com", "user_id": "user_789"},
		"template_id": "welcome",
		"template_data": {"name": "Anna"},
		"metadata": {"correlation_id": "order_456"}
	}`)

	require.Equal(t, http.StatusAccepted, rec.Code)

	var resp api.NotificationAccepted
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Id)
	require.Equal(t, api.Pending, resp.Status)
	require.False(t, resp.CreatedAt.IsZero())

	require.NotNil(t, published)
	require.Equal(t, resp.Id, published.ID)
	require.Equal(t, domainnotification.PriorityTransactional, published.Priority,
		"omitted priority must default to transactional")
}

func TestSendNotificationValidationFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl) // no EXPECT: nothing may be published

	server := newTestServer(t, publisher)

	rec := doRequest(t, server, http.MethodPost, "/api/v1/notifications", `{
		"type": "email",
		"recipient": {"email": "not-an-email"},
		"template_id": "welcome"
	}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "validation_error", resp.Code)
	require.NotNil(t, resp.Field)
	require.Equal(t, "recipient.email", *resp.Field)
}

func TestSendNotificationUnknownTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)

	server := newTestServer(t, publisher)

	rec := doRequest(t, server, http.MethodPost, "/api/v1/notifications", `{
		"type": "email",
		"recipient": {"email": "anna.kowalska@example.com"},
		"template_id": "order_confirmation"
	}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "template_not_found", resp.Code)
}

func TestSendNotificationRejectedBySpecValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing required type",
			body: `{"recipient": {"email": "anna.kowalska@example.com"}}`,
		},
		{
			name: "unknown priority enum value",
			body: `{"type": "email", "priority": "urgent", "recipient": {"email": "anna.kowalska@example.com"}}`,
		},
		{
			name: "malformed json",
			body: `{"type": "email",`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			publisher := mocks.NewMockPublisher(ctrl) // no EXPECT: must be rejected before the domain
			server := newTestServer(t, publisher)

			rec := doRequest(t, server, http.MethodPost, "/api/v1/notifications", tt.body)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestSendNotificationPublishFailureIsOpaque(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)
	publisher.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		Return(errors.New("amqp://user:secret@rabbitmq connection refused"))

	server := newTestServer(t, publisher)

	rec := doRequest(t, server, http.MethodPost, "/api/v1/notifications", `{
		"type": "email",
		"recipient": {"email": "anna.kowalska@example.com"},
		"template_id": "welcome"
	}`)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "internal_error", resp.Code)
	require.NotContains(t, rec.Body.String(), "rabbitmq", "broker details must not leak to clients")
}

func TestSendNotificationBatchPartialSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := mocks.NewMockPublisher(ctrl)
	publisher.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	server := newTestServer(t, publisher)

	rec := doRequest(t, server, http.MethodPost, "/api/v1/notifications/batch", `{
		"notifications": [
			{"type": "email", "recipient": {"email": "anna.kowalska@example.com"}, "template_id": "welcome"},
			{"type": "email", "recipient": {"email": "not-an-email"}, "template_id": "welcome"}
		]
	}`)

	require.Equal(t, http.StatusAccepted, rec.Code)

	var resp api.SendNotificationBatchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 2)

	require.Equal(t, 0, resp.Results[0].Index)
	require.NotNil(t, resp.Results[0].Notification)
	require.Nil(t, resp.Results[0].Error)

	require.Equal(t, 1, resp.Results[1].Index)
	require.Nil(t, resp.Results[1].Notification)
	require.NotNil(t, resp.Results[1].Error)
	require.Equal(t, "validation_error", resp.Results[1].Error.Code)
}

func TestListTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mocks.NewMockPublisher(ctrl))

	tests := []struct {
		name      string
		path      string
		wantCount int
	}{
		{name: "all templates", path: "/api/v1/templates", wantCount: 3},
		{name: "filtered by type", path: "/api/v1/templates?type=email", wantCount: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRequest(t, server, http.MethodGet, tt.path, "")

			require.Equal(t, http.StatusOK, rec.Code)

			var resp api.TemplateList
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Len(t, resp.Templates, tt.wantCount)
		})
	}
}

func TestListTemplatesInvalidTypeFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mocks.NewMockPublisher(ctrl))

	rec := doRequest(t, server, http.MethodGet, "/api/v1/templates?type=fax", "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mocks.NewMockPublisher(ctrl))

	rec := doRequest(t, server, http.MethodGet, "/api/v1/templates/email/welcome", "")

	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.Template
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "welcome", resp.Id)
	require.Equal(t, api.Email, resp.Type)
	require.Equal(t, "Welcome Email", resp.Name)
	require.False(t, resp.IsDefault)
	require.Equal(t, "Welcome, {{.name}}!", resp.Subject)
	require.Contains(t, resp.Body, "<h1>Hello, {{.name}}!</h1>")
}

func TestGetTemplateNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mocks.NewMockPublisher(ctrl))

	rec := doRequest(t, server, http.MethodGet, "/api/v1/templates/email/order_confirmation", "")

	require.Equal(t, http.StatusNotFound, rec.Code)

	var resp api.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "template_not_found", resp.Code)
}

func TestAccessLogRecordsRealStatus(t *testing.T) {
	ctrl := gomock.NewController(t)

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "email"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "email", "welcome.html"), []byte(welcomeEmailTemplate), 0o644))
	repo, err := infratemplate.NewFileRepository(dir)
	require.NoError(t, err)

	var logBuf strings.Builder
	logger := zerolog.New(&logBuf)
	tmplSvc := apptemplate.NewService(repo)
	notifSvc := appnotification.NewService(mocks.NewMockPublisher(ctrl), tmplSvc)
	server, err := NewServer(notifSvc, tmplSvc, logger)
	require.NoError(t, err)

	rec := doRequest(t, server, http.MethodGet, "/api/v1/templates/email/missing", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, logBuf.String(), `"status":404`,
		"access log must record the status actually sent to the client")
}

func TestHealthCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mocks.NewMockPublisher(ctrl))

	rec := doRequest(t, server, http.MethodGet, "/health", "")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "healthy")
}
