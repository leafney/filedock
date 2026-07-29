/**
 * @Author:      leafney
 * @GitHub:      https://github.com/leafney
 * @Project:     grape
 * @Date:        2024-09-15 02:22
 * @Description:
 */

package utils

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// 清除 s 中特殊的不可见字符
func GraphicString(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
}

// IconURLToBase64 将图标URL转换为Base64编码的图片字符串
func IconURLToBase64(url string, timeout time.Duration) (string, error) {
	// 创建一个带有超时的 HTTP 客户端
	client := &http.Client{
		Timeout: timeout,
	}

	// 创建一个新的 HTTP GET 请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("无法创建请求: %v", err)
	}

	// 添加常见的 User-Agent 请求头，模拟浏览器
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	// 尝试添加 Referer 请求头，使用 URL 本身作为 Referer (可能需要根据实际情况调整)
	req.Header.Set("Referer", url)

	// 发送 HTTP GET 请求获取图标
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("无法获取 URL: %v", err)
	}
	defer resp.Body.Close() // 确保响应 body 在函数退出时关闭

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败，状态码: %s", resp.Status)
	}

	// 获取 Content-Type 并检查是否为图片类型
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("URL 内容不是图片: %s", contentType)
	}

	// 读取响应 body 的内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("无法读取响应内容: %v", err)
	}

	// 将图片数据转换为 Base64 编码
	encoded := base64.StdEncoding.EncodeToString(data)

	// 构造完整的 Base64 字符串，包括 MIME 类型
	base64Str := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
	return base64Str, nil
}

// FormatBookmarkTags 格式化书签tag字符串，替换全角逗号为半角逗号并处理空格，最后通过指定分隔符连接
func FormatBookmarkTags(tags string, separator string) string {
	// 设置默认分隔符为空格
	sep := " "
	// 如果提供了自定义分隔符，则使用它
	if len(separator) > 0 {
		sep = separator
	}

	// 替换全角逗号为半角逗号
	tags = strings.ReplaceAll(tags, "，", ",")
	// 去除前后空格
	tags = strings.TrimSpace(tags)
	// 分割并重新组合，去除多余空格
	tagList := strings.Split(tags, ",")

	// 过滤空标签并去除多余空格
	var filteredTags []string
	for _, tag := range tagList {
		trimmedTag := strings.TrimSpace(tag)
		if trimmedTag != "" {
			filteredTags = append(filteredTags, trimmedTag)
		}
	}

	// 使用指定的分隔符连接标签
	return strings.Join(filteredTags, sep)
}

// 去除重复字符串
func RemoveDuplicateStrings(strSlice []string) []string {
	encountered := make(map[string]struct{}, len(strSlice))
	result := make([]string, 0, len(strSlice))

	for _, str := range strSlice {
		if _, exists := encountered[str]; !exists {
			encountered[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}

// IsBase64Icon 判断图标字符串是否为base64格式
func IsBase64Icon(icon string) bool {
	// 检查是否为空
	if icon == "" {
		return false
	}
	// 检查是否以data:开头，这是base64编码图片的特征
	return strings.HasPrefix(icon, "data:")
}

// IsURLIcon 判断图标字符串是否为URL链接
func IsURLIcon(icon string) bool {
	// 检查是否为空
	if icon == "" {
		return false
	}
	// 检查是否以http://或https://开头
	return strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://")
}
