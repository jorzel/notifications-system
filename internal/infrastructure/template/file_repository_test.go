package template

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jorzel/notifications-system/internal/domain/notification"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

const welcomeEmailFile = `---
name: Welcome Email
subject: Welcome, {{.name}}!
---
<html><body><h1>Hello, {{.name}}!</h1></body></html>
`

const defaultEmailFile = `---
subject: "{{.subject}}"
---
<html><body><p>{{.message}}</p></body></html>
`

const defaultSMSFile = `Your verification code is {{.code}}.`

func writeTemplateDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	return dir
}

func TestFileRepositoryGetSuccess(t *testing.T) {
	dir := writeTemplateDir(t, map[string]string{
		"email/welcome.html": welcomeEmailFile,
		"sms/default.txt":    defaultSMSFile,
	})
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)

	tmpl, err := repo.Get(context.Background(), notification.TypeEmail, "welcome")

	require.NoError(t, err)
	require.Equal(t, "welcome", tmpl.ID)
	require.Equal(t, notification.TypeEmail, tmpl.Type)
	require.Equal(t, "Welcome Email", tmpl.Name)
	require.False(t, tmpl.IsDefault)

	content, err := tmpl.Render(map[string]any{"name": "Anna"})
	require.NoError(t, err)
	require.Equal(t, "Welcome, Anna!", content.Subject)
	require.Contains(t, content.Body, "<h1>Hello, Anna!</h1>")
	require.NotContains(t, content.Body, "---", "front matter must not leak into the body")
}

func TestFileRepositoryGetWithoutFrontMatter(t *testing.T) {
	dir := writeTemplateDir(t, map[string]string{
		"sms/default.txt": defaultSMSFile,
	})
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)

	tmpl, err := repo.Get(context.Background(), notification.TypeSMS, "default")

	require.NoError(t, err)
	require.Equal(t, "default", tmpl.Name, "name falls back to the template ID")
	require.True(t, tmpl.IsDefault)

	content, err := tmpl.Render(map[string]any{"code": "839201"})
	require.NoError(t, err)
	require.Empty(t, content.Subject)
	require.Equal(t, "Your verification code is 839201.", content.Body)
}

func TestFileRepositoryGetNotFound(t *testing.T) {
	dir := writeTemplateDir(t, map[string]string{
		"email/welcome.html": welcomeEmailFile,
	})
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)

	tests := []struct {
		name       string
		notifType  notification.Type
		templateID string
	}{
		{name: "unknown id", notifType: notification.TypeEmail, templateID: "order_confirmation"},
		{name: "id exists only under another type", notifType: notification.TypeSMS, templateID: "welcome"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.Get(context.Background(), tt.notifType, tt.templateID)

			require.ErrorIs(t, err, domaintemplate.ErrTemplateNotFound)
		})
	}
}

func TestFileRepositoryGetDefaultSuccess(t *testing.T) {
	dir := writeTemplateDir(t, map[string]string{
		"email/default.html": defaultEmailFile,
		"email/welcome.html": welcomeEmailFile,
	})
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)

	tmpl, err := repo.GetDefault(context.Background(), notification.TypeEmail)

	require.NoError(t, err)
	require.Equal(t, "default", tmpl.ID)
	require.Equal(t, notification.TypeEmail, tmpl.Type)
	require.True(t, tmpl.IsDefault)
}

func TestFileRepositoryGetDefaultNotFound(t *testing.T) {
	dir := writeTemplateDir(t, map[string]string{
		"email/welcome.html": welcomeEmailFile,
	})
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)

	_, err = repo.GetDefault(context.Background(), notification.TypeEmail)

	require.ErrorIs(t, err, domaintemplate.ErrNoDefaultTemplate)
}

func TestFileRepositoryList(t *testing.T) {
	dir := writeTemplateDir(t, map[string]string{
		"email/default.html": defaultEmailFile,
		"email/welcome.html": welcomeEmailFile,
		"sms/default.txt":    defaultSMSFile,
	})
	repo, err := NewFileRepository(dir)
	require.NoError(t, err)

	templates, err := repo.List(context.Background())

	require.NoError(t, err)
	require.Len(t, templates, 3)

	ids := make([]string, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, string(tmpl.Type)+"/"+tmpl.ID)
	}
	require.ElementsMatch(t, []string{"email/default", "email/welcome", "sms/default"}, ids)
}

func TestNewFileRepositoryFailed(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name: "unknown notification type directory",
			files: map[string]string{
				"fax/default.txt": "irrelevant",
			},
		},
		{
			name: "unclosed front matter",
			files: map[string]string{
				"email/welcome.html": "---\nsubject: Welcome\n<html></html>",
			},
		},
		{
			name: "invalid yaml in front matter",
			files: map[string]string{
				"email/welcome.html": "---\nsubject: [unbalanced\n---\n<html></html>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTemplateDir(t, tt.files)

			_, err := NewFileRepository(dir)

			require.Error(t, err)
		})
	}
}

func TestNewFileRepositoryMissingDir(t *testing.T) {
	_, err := NewFileRepository(filepath.Join(t.TempDir(), "does-not-exist"))

	require.Error(t, err)
}
