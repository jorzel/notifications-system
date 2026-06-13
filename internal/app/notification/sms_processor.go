package notification

import (
	"context"
	"fmt"

	"golang.org/x/time/rate"

	domainnotification "github.com/jorzel/notifications-system/internal/domain/notification"
	"github.com/jorzel/notifications-system/internal/domain/provider"
)

// SMSProcessor renders an SMS notification and delivers it via the
// SMSSender, rate-limiting outbound calls.
type SMSProcessor struct {
	renderer *Renderer
	sender   provider.SMSSender
	limiter  *rate.Limiter
}

// NewSMSProcessor creates an SMSProcessor.
func NewSMSProcessor(renderer *Renderer, sender provider.SMSSender, limiter *rate.Limiter) *SMSProcessor {
	return &SMSProcessor{renderer: renderer, sender: sender, limiter: limiter}
}

// Process renders the template and sends the SMS. The rendered Body is the
// message text. Recipient and content failures are permanent; a send
// failure is transient and retried.
func (p *SMSProcessor) Process(ctx context.Context, msg *domainnotification.Message) error {
	content, err := p.renderer.Render(ctx, msg)
	if err != nil {
		return err
	}

	phone, err := provider.NewPhoneNumber(msg.Recipient.PhoneNumber)
	if err != nil {
		return newPermanentError(fmt.Errorf("invalid phone recipient: %w", err))
	}

	sms, err := provider.NewSMSMessage(phone, content.Body)
	if err != nil {
		return newPermanentError(fmt.Errorf("build sms message: %w", err))
	}

	if err := p.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter wait: %w", err)
	}

	if err := p.sender.Send(ctx, sms); err != nil {
		return fmt.Errorf("send sms: %w", err)
	}
	return nil
}
