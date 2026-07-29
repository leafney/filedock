package captchax

import "github.com/mojocn/base64Captcha"

// GenerateDigitCaptcha 生成 4 位数字验证码
// 返回: id (无需使用), b64s (Base64图片), answer (答案), error
func GenerateDigitCaptcha() (string, string, string, error) {
	driver := base64Captcha.NewDriverDigit(80, 240, 4, 0.7, 80)
	id, content, answer := driver.GenerateIdQuestionAnswer()
	item, err := driver.DrawCaptcha(content)
	if err != nil {
		return "", "", "", err
	}

	return id, item.EncodeB64string(), answer, nil
}
