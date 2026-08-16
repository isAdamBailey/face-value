// Package email sends transactional emails, such as magic-link sign-in
// links.
package email

import "context"

// Sender sends transactional emails.
type Sender interface {
	// SendMagicLink emails a sign-in link to toEmail.
	SendMagicLink(ctx context.Context, toEmail, link string) error
}

const magicLinkSubject = "Your Face Value sign-in link"

func magicLinkBody(link string) (text, html string) {
	text = "Click the link below to sign in to Face Value. This link expires in 15 minutes and can only be used once.\n\n" + link
	html = `<p>Click the link below to sign in to Face Value. This link expires in 15 minutes and can only be used once.</p><p><a href="` + link + `">` + link + `</a></p>`
	return text, html
}
