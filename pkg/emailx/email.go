package emailx

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

// SMTPConfig SMTP 配置
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// SendEmail 发送邮件
func SendEmail(cfg *SMTPConfig, to, subject, body string) error {
	// 解析端口
	port, err := strconv.Atoi(cfg.Port)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %v", err)
	}

	// 构建邮件内容
	message := []byte(
		"From: " + cfg.From + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	// SMTP 认证
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)

	// 如果端口是465，使用SSL/TLS
	if port == 465 {
		return sendMailTLS(addr, auth, cfg.From, []string{to}, message)
	}

	// 否则使用STARTTLS
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, message)
}

// sendMailTLS 使用 TLS 发送邮件（端口465）
func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// 创建 TLS 连接
	tlsConfig := &tls.Config{
		ServerName:         strings.Split(addr, ":")[0],
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, tlsConfig.ServerName)
	if err != nil {
		return err
	}
	defer client.Quit()

	// 认证
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	// 设置发件人
	if err = client.Mail(from); err != nil {
		return err
	}

	// 设置收件人
	for _, addr := range to {
		if err = client.Rcpt(addr); err != nil {
			return err
		}
	}

	// 发送邮件内容
	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// BuildCaptchaEmailBody 构建验证码邮件内容
func BuildCaptchaEmailBody(captcha string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>验证码</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #ddd; border-radius: 5px;">
        <h2 style="color: #4CAF50;">Zhuque 验证码</h2>
        <p>您好！</p>
        <p>您的验证码是：</p>
        <div style="background-color: #f4f4f4; padding: 15px; text-align: center; font-size: 24px; font-weight: bold; color: #4CAF50; letter-spacing: 5px; border-radius: 5px;">
            %s
        </div>
        <p style="color: #666; font-size: 14px;">验证码有效期为 5 分钟，请尽快使用。</p>
        <p style="color: #999; font-size: 12px; margin-top: 30px;">如果这不是您的操作，请忽略此邮件。</p>
        <hr style="border: none; border-top: 1px solid #ddd; margin: 20px 0;">
        <p style="color: #999; font-size: 12px;">此邮件由 Zhuque 分布式定时任务系统自动发送，请勿回复。</p>
    </div>
</body>
</html>
`, captcha)
}
