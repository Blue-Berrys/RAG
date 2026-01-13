package llm

import (
	"fmt"
	"os"
	"testing"
)

// TestValidateAPIKey 测试API Key验证功能
func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "空API Key",
			apiKey:  "",
			wantErr: true,
			errMsg:  "API key cannot be empty",
		},
		{
			name:    "太短的API Key",
			apiKey:  "short.key",
			wantErr: true,
			errMsg:  "too short",
		},
		{
			name:    "缺少点分隔符",
			apiKey:  "abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			wantErr: true,
			errMsg:  "expected format",
		},
		{
			name:    "有效的API Key格式",
			apiKey:  "1234567890.abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890",
			wantErr: false,
		},
		{
			name:    "带空格的有效API Key",
			apiKey:  "  1234567890.abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890  ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAPIKey(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAPIKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErr {
				// 检查错误消息是否包含预期的内容
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("validateAPIKey() error message = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestMaskAPIKey 测试API Key掩码功能
func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "空API Key",
			apiKey:   "",
			expected: "<empty>",
		},
		{
			name:     "短API Key",
			apiKey:   "short",
			expected: "***",
		},
		{
			name:     "标准格式API Key",
			apiKey:   "1234567890.abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890",
			expected: "1234***.abcd...7890",
		},
		{
			name:     "非标准格式API Key",
			apiKey:   "abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJ",
			expected: "abcd...GHIJ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAPIKey(tt.apiKey)
			if result != tt.expected {
				t.Errorf("maskAPIKey() = %v, want %v", result, tt.expected)
			}
			// 确保掩码后的字符串不包含完整的原始密钥
			if tt.apiKey != "" && tt.apiKey != "short" && len(tt.apiKey) > 8 {
				if containsString(result, tt.apiKey[5:len(tt.apiKey)-5]) {
					t.Errorf("maskAPIKey() result contains too much of the original key")
				}
			}
		})
	}
}

// TestNewZhipuLLM_InvalidKey 测试使用无效API Key创建LLM
func TestNewZhipuLLM_InvalidKey(t *testing.T) {
	// 保存原始环境变量
	originalKey := os.Getenv("ZHIPU_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ZHIPU_API_KEY", originalKey)
		} else {
			os.Unsetenv("ZHIPU_API_KEY")
		}
	}()

	// 测试无效的API Key
	os.Setenv("ZHIPU_API_KEY", "invalid.key")
	_, err := NewZhipuLLM("glm-4-flash")
	if err == nil {
		t.Errorf("NewZhipuLLM() expected error for invalid API key, got nil")
	}
}

// 辅助函数：检查字符串是否包含子字符串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Example_validateAPIKey 示例：验证API Key
func Example_validateAPIKey() {
	// 有效的智谱AI API Key格式
	validKey := "1234567890.abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	err := validateAPIKey(validKey)
	if err != nil {
		fmt.Println("Invalid:", err)
	} else {
		fmt.Println("Valid API Key")
	}
	// Output: Valid API Key
}

// Example_maskAPIKey 示例：掩码API Key
func Example_maskAPIKey() {
	apiKey := "1234567890.abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	masked := maskAPIKey(apiKey)
	fmt.Println("Masked:", masked)
	// Output: Masked: 1234***.abcd...7890
}
