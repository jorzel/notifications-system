// Package template provides a file-based implementation of the domain
// template.Repository.
//
// Templates are loaded once at construction from a directory tree where
// each subdirectory is a notification type and each file is a template:
//
//	templates/
//	├── email/
//	│   ├── default.html
//	│   └── welcome.html
//	├── sms/
//	│   └── default.txt
//	└── push/
//	    └── default.txt
//
// The template ID is the filename without extension; the file named
// "default" is the default template for its type. A file may start with
// an optional YAML front matter block delimited by "---" lines:
//
//	---
//	name: Welcome Email
//	subject: Welcome, {{.name}}!
//	---
//	<html>...
//
// "subject" is the Go template for the email subject or push title,
// "name" is a human-readable label (defaults to the template ID).
// Everything after the closing delimiter is the template body.
package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jorzel/notifications-system/internal/domain/notification"
	domaintemplate "github.com/jorzel/notifications-system/internal/domain/template"
)

const (
	defaultTemplateID = "default"
	frontMatterDelim  = "---"
)

// FileRepository serves templates loaded from a directory tree.
// All templates are read at construction; lookups are in-memory.
type FileRepository struct {
	templates map[notification.Type]map[string]*domaintemplate.Template
}

// NewFileRepository loads all templates under rootDir.
// It fails fast on an unknown type directory, unreadable file, or
// malformed front matter, so configuration errors surface at startup.
func NewFileRepository(rootDir string) (*FileRepository, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	repo := &FileRepository{
		templates: make(map[notification.Type]map[string]*domaintemplate.Template),
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		notifType := notification.Type(entry.Name())
		if !notifType.IsValid() {
			return nil, fmt.Errorf("unknown notification type directory %q in %s", entry.Name(), rootDir)
		}

		if err := repo.loadTypeDir(notifType, filepath.Join(rootDir, entry.Name())); err != nil {
			return nil, err
		}
	}

	return repo, nil
}

// Get retrieves a template by notification type and ID.
func (r *FileRepository) Get(_ context.Context, notificationType notification.Type, id string) (*domaintemplate.Template, error) {
	tmpl, ok := r.templates[notificationType][id]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", domaintemplate.ErrTemplateNotFound, notificationType, id)
	}
	return tmpl, nil
}

// GetDefault retrieves the default template for a notification type.
func (r *FileRepository) GetDefault(_ context.Context, notificationType notification.Type) (*domaintemplate.Template, error) {
	tmpl, ok := r.templates[notificationType][defaultTemplateID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", domaintemplate.ErrNoDefaultTemplate, notificationType)
	}
	return tmpl, nil
}

// List retrieves all templates, ordered by type then ID.
func (r *FileRepository) List(_ context.Context) ([]*domaintemplate.Template, error) {
	var templates []*domaintemplate.Template
	for _, byID := range r.templates {
		for _, tmpl := range byID {
			templates = append(templates, tmpl)
		}
	}

	sort.Slice(templates, func(i, j int) bool {
		if templates[i].Type != templates[j].Type {
			return templates[i].Type < templates[j].Type
		}
		return templates[i].ID < templates[j].ID
	})

	return templates, nil
}

func (r *FileRepository) loadTypeDir(notifType notification.Type, dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read template directory %s: %w", dir, err)
	}

	r.templates[notifType] = make(map[string]*domaintemplate.Template, len(files))

	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		tmpl, err := loadTemplateFile(notifType, filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}
		r.templates[notifType][tmpl.ID] = tmpl
	}

	return nil
}

func loadTemplateFile(notifType notification.Type, path string) (*domaintemplate.Template, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", path, err)
	}

	meta, body, err := splitFrontMatter(string(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid front matter in %s: %w", path, err)
	}

	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	name := meta.Name
	if name == "" {
		name = id
	}

	return domaintemplate.New(id, notifType, name, meta.Subject, body, id == defaultTemplateID), nil
}

// frontMatter is the optional YAML metadata block at the top of a template file.
type frontMatter struct {
	Name    string `yaml:"name"`
	Subject string `yaml:"subject"`
}

// splitFrontMatter separates the optional front matter block from the body.
// Content without a leading "---" line is returned unchanged as the body.
func splitFrontMatter(content string) (frontMatter, string, error) {
	var meta frontMatter

	if !strings.HasPrefix(content, frontMatterDelim+"\n") {
		return meta, content, nil
	}

	rest := content[len(frontMatterDelim)+1:]
	end := strings.Index(rest, "\n"+frontMatterDelim)
	if end == -1 {
		return meta, "", fmt.Errorf("missing closing %q delimiter", frontMatterDelim)
	}

	if err := yaml.Unmarshal([]byte(rest[:end]), &meta); err != nil {
		return meta, "", err
	}

	body := rest[end+1+len(frontMatterDelim):]
	body = strings.TrimPrefix(body, "\n")

	return meta, body, nil
}
