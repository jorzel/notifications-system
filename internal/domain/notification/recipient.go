package notification

// Recipient holds the delivery address for a notification.
// Only one of Email, PhoneNumber, or DeviceToken should be set
// depending on the notification type.
type Recipient struct {
	// Email address for email notifications.
	Email string

	// PhoneNumber in E.164 format for SMS notifications.
	PhoneNumber string

	// DeviceToken for push notifications (FCM/APNs).
	DeviceToken string

	// UserID is an optional reference to the user receiving the notification.
	UserID string
}

// NewEmailRecipient creates a recipient for email notifications.
func NewEmailRecipient(email, userID string) Recipient {
	return Recipient{
		Email:  email,
		UserID: userID,
	}
}

// NewSMSRecipient creates a recipient for SMS notifications.
func NewSMSRecipient(phoneNumber, userID string) Recipient {
	return Recipient{
		PhoneNumber: phoneNumber,
		UserID:      userID,
	}
}

// NewPushRecipient creates a recipient for push notifications.
func NewPushRecipient(deviceToken, userID string) Recipient {
	return Recipient{
		DeviceToken: deviceToken,
		UserID:      userID,
	}
}
