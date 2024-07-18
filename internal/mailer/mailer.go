package mailer

import (
	"embed"
	"html/template"
	"time"

	"github.com/wneessen/go-mail"
)

//go:embed "html"
var mailFS embed.FS

type Mailer struct {
	Client *mail.Client
}

func New(host, username, password string) (*Mailer, error) {
	mailer := &Mailer{}

	client, err := mail.NewClient(
		host,
		mail.WithUsername(username),
		mail.WithPassword(password),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithTLSPortPolicy(mail.TLSMandatory),
	)
	if err != nil {
		return nil, err
	}

	mailer.Client = client
	return mailer, nil
}

func (m *Mailer) Send(sender, recipient, templateFile string, data interface{}) error {
	msg := mail.NewMsg()

	if err := msg.From(sender); err != nil {
		return err
	}

	if err := msg.To(recipient); err != nil {
		return err
	}

	tmpl, err := template.New(templateFile).ParseFS(mailFS, "html/"+templateFile)
	if err != nil {
		return err
	}

	msg.Subject("Welcome to the forum!")
	if err := msg.SetBodyHTMLTemplate(tmpl, data); err != nil {
		return err
	}

	for i := 1; i < 3; i++ {
		err := m.Client.DialAndSend(msg)
		if nil == err {
			return nil
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return err
}
