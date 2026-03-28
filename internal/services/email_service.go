package services

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"gopkg.in/gomail.v2"
)

type EmailService struct {
	fromEmail    string
	fromPassword string
	smtpHost     string
	smtpPort     int
}

func NewEmailService() *EmailService {
	// В реальном проекте бери из конфига
	return &EmailService{
		fromEmail:    os.Getenv("SMTP_EMAIL"),
		fromPassword: os.Getenv("SMTP_PASSWORD"),
		smtpHost:     os.Getenv("SMTP_HOST"),
		smtpPort:     587, // Для Gmail, Outlook и др.
	}
}

// GenerateVerificationCode генерирует 6-значный код
func (s *EmailService) GenerateVerificationCode() (string, error) {
	max := 999999
	min := 100000

	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	code := min + (int(b[0])<<24|int(b[1])<<16|int(b[2])<<8|int(b[3]))%(max-min+1)
	return fmt.Sprintf("%06d", code), nil
}

// SendVerificationEmail отправляет код подтверждения
func (s *EmailService) SendVerificationEmail(toEmail, code, firstName string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", s.fromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", "Подтверждение email - CloudStorage")

	// HTML шаблон письма
	body := fmt.Sprintf(`
    <!DOCTYPE html>
    <html>
    <head>
        <style>
            body { font-family: Arial, sans-serif; background: #0d1117; color: #c9d1d9; padding: 20px; }
            .container { max-width: 600px; margin: 0 auto; background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 30px; }
            .header { font-size: 24px; color: #58a6ff; margin-bottom: 20px; }
            .code { font-size: 32px; font-weight: bold; color: #58a6ff; text-align: center; padding: 20px; background: #0d1117; border-radius: 8px; margin: 20px 0; }
            .footer { font-size: 12px; color: #8b949e; margin-top: 20px; }
        </style>
    </head>
    <body>
        <div class="container">
            <div class="header">☁️ CloudStorage</div>
            <h2>Привет, %s!</h2>
            <p>Для подтверждения email введите следующий код:</p>
            <div class="code">%s</div>
            <p>Код действителен в течение 1 часа.</p>
            <p>Если вы не регистрировались на CloudStorage, просто проигнорируйте это письмо.</p>
            <div class="footer">© 2026 CloudStorage. Все права защищены.</div>
        </div>
    </body>
    </html>
    `, firstName, code)

	m.SetBody("text/html", body)

	d := gomail.NewDialer(s.smtpHost, s.smtpPort, s.fromEmail, s.fromPassword)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send email: %v", err)
		return err
	}

	log.Printf("Verification email sent to %s", toEmail)
	return nil
}
