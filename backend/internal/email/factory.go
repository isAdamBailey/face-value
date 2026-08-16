package email

import "fmt"

// Config holds the settings needed to construct a Sender.
type Config struct {
	Provider     string // "smtp" or "ses"
	FromEmail    string
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
}

// New constructs a Sender based on cfg.Provider.
func New(cfg Config) (Sender, error) {
	switch cfg.Provider {
	case "smtp", "ses":
		return NewSMTPSender(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.FromEmail), nil
	default:
		return nil, fmt.Errorf("unknown email provider %q", cfg.Provider)
	}
}
