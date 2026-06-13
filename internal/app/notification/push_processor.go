package notification

import (
	"context"
	"fmt"

	"golang.org/x/time/rate"

	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/provider"
)

// PushProcessor renders a push notification and delivers it via the
// PushSender, rate-limiting outbound calls.
type PushProcessor struct {
	renderer *Renderer
	sender   provider.PushSender
	limiter  *rate.Limiter
}

// NewPushProcessor creates a PushProcessor.
func NewPushProcessor(renderer *Renderer, sender provider.PushSender, limiter *rate.Limiter) *PushProcessor {
	return &PushProcessor{renderer: renderer, sender: sender, limiter: limiter}
}

// Process renders the template and sends the push notification. The
// rendered Subject is the title, Body the body, Data the extra payload.
// Recipient and content failures are permanent; a send failure is
// transient and retried.
func (p *PushProcessor) Process(ctx context.Context, msg *domainnotification.Message) error {
	content, err := p.renderer.Render(ctx, msg)
	if err != nil {
		return err
	}

	token, err := provider.NewDeviceToken(msg.Recipient.DeviceToken)
	if err != nil {
		return newPermanentError(fmt.Errorf("invalid device token: %w", err))
	}

	push, err := provider.NewPushMessage(token, content.Subject, content.Body, content.Data)
	if err != nil {
		return newPermanentError(fmt.Errorf("build push message: %w", err))
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait: %w", err)
	}

	if err := p.sender.Send(ctx, push); err != nil {
		return fmt.Errorf("send push: %w", err)
	}
	return nil
}
