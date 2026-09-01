// guesswhat: 穷举 URL 中以 '*' 占位的未知字符。
// 每隔一秒发送一个请求：响应是 Base64 字符串视为命中，
// 响应是 JSON 且包含 "error" 视为 URL 错误，继续穷举。
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ==================== 在这里填写要穷举的 URL ====================
// 未知的字符用 '*' 代替，例如: "http://example.com/api/****"
const urlTemplate = "https://curl-a.microsucon.org/api/v1/client/subscribe?token=aJ5fGECg6ou7CbhmvZmL1**FuMdu****"

// ==================== 其他常量 ====================
const (
	outFile         = "result.txt"     // 记录正确 URL 的文本文件
	requestInterval = time.Second      // 每隔一秒发送一个请求
	requestTimeout  = 10 * time.Second // 单个请求的超时时间
	minBase64Len    = 8                // 视为 Base64 正确响应的最小长度（过滤过短的误判）
)

// 星号占位符可取的字符：大写字母、小写字母、数字
const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func main() {
	var positions []int
	for i := 0; i < len(urlTemplate); i++ {
		if urlTemplate[i] == '*' {
			positions = append(positions, i)
		}
	}
	if len(positions) == 0 {
		fmt.Fprintln(os.Stderr, "URL 中没有 '*' 占位符，无需穷举")
		os.Exit(2)
	}
	if len(positions) > 10 {
		fmt.Fprintf(os.Stderr, "占位符数量 %d 过多，组合数超出可枚举范围，请减少占位符或提供更精确的 URL\n", len(positions))
		os.Exit(2)
	}

	var total uint64 = 1
	for range positions {
		total *= uint64(len(charset))
	}
	if len(positions) > 3 {
		fmt.Fprintf(os.Stderr, "提示: 共 %d 个占位符，候选组合 %d 个，按每秒 1 个请求计算需要非常长的时间，请尽量提供更精确的 URL\n", len(positions), total)
	}
	fmt.Printf("占位符 %d 个，候选 URL 共 %d 个，每隔一秒尝试一个，开始穷举...\n", len(positions), total)

	out, err := os.OpenFile(outFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开结果文件失败: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	client := &http.Client{Timeout: requestTimeout}
	ctx := context.Background()

	for idx := uint64(0); idx < total; idx++ {
		u := buildURL(urlTemplate, positions, idx)
		fmt.Printf("[%d/%d] 尝试: %s ... ", idx+1, total, u)

		if ok, body := fetch(ctx, client, u, minBase64Len); ok {
			fmt.Printf("命中!\n响应: %s", body)
			if dec, err := decodeBase64(bytes.TrimSpace(body)); err == nil {
				fmt.Printf("\nBase64 解码: %s", dec)
			}
			fmt.Println()
			fmt.Fprintf(out, "%s\n", u)
			fmt.Printf("正确的 URL 已记录到 %s\n", outFile)
			return
		}
		fmt.Println("错误")

		if idx+1 < total {
			time.Sleep(requestInterval)
		}
	}
	fmt.Println("穷举完成，未找到正确的 URL")
}

// buildURL 用第 idx 个组合（62 进制展开）填充模板中的所有 '*'。
// idx 从 0 到 62^n-1 遍历即可覆盖全部组合且互不重复。
func buildURL(template string, positions []int, idx uint64) string {
	buf := []byte(template)
	v := idx
	for _, p := range positions {
		buf[p] = charset[v%uint64(len(charset))]
		v /= uint64(len(charset))
	}
	return string(buf)
}

// fetch 请求候选 URL，返回 (是否命中, 响应体)。
// 请求失败一律按错误处理，继续穷举。
func fetch(ctx context.Context, client *http.Client, u string, minLen int) (bool, []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, nil
	}
	return isCorrect(body, minLen), bytes.TrimSpace(body)
}

// isCorrect 判定响应是否为正确的值：
//   - JSON 格式（尤其含 "error"）=> 错误
//   - 长度足够的 Base64 字符串 => 正确
//   - 其他 => 错误
func isCorrect(body []byte, minLen int) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || len(trimmed) < minLen {
		return false
	}
	if json.Valid(trimmed) {
		return false
	}
	return isBase64(trimmed)
}

func isBase64(s []byte) bool {
	_, err := decodeBase64(s)
	return err == nil
}

// decodeBase64 尝试标准/无填充/URL 安全等多种 Base64 变体。
func decodeBase64(s []byte) ([]byte, error) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		if dec, err := enc.DecodeString(string(s)); err == nil {
			return dec, nil
		}
	}
	return nil, errors.New("not base64")
}
