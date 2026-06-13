package notification

import (
	"context"
	"fmt"

	"golang.org/x/time/rate"

	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/provider"
)

// EmailProcessor renders an email notification and delivers it via the
// EmailSender, rate-limiting outbound calls.
type EmailProcessor struct {
	renderer *Renderer
	sender   provider.EmailSender
	limiter  *rate.Limiter
}

// NewEmailProcessor creates an EmailProcessor.
func NewEmailProcessor(renderer *Renderer, sender provider.EmailSender, limiter *rate.Limiter) *EmailProcessor {
	return &EmailProcessor{renderer: renderer, sender: sender, limiter: limiter}
}

// Process renders the template and sends the email. Recipient and content
// failures are permanent; a send failure is transient and retried.
func (p *EmailProcessor) Process(ctx context.Context, msg *domainnotification.Message) error {
	content, err := p.renderer.Render(ctx, msg)
	if err != nil {
		return err
	}

	addr, err := provider.NewEmailAddress(msg.Recipient.Email)
	if err != nil {
		return newPermanentError(fmt.Errorf("invalid email recipient: %w", err))
	}

	email, err := provider.NewEmailMessage(addr, content.Subject, content.Body)
	if err != nil {
		return newPermanentError(fmt.Errorf("build email message: %w", err))
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait: %w", err)
	}

	if err := p.sender.Send(ctx, email); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
