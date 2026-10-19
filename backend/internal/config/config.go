package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	Server struct {
		Host       string `json:"host"` // 监听地址：前置 nginx/Caddy 时建议 127.0.0.1；留空监听全部网卡（兼容旧配置）
		Port       string `json:"port"`
		TrustProxy bool   `json:"trust_proxy"` // 是否信任反向代理传递的客户端 IP（前置 nginx/Caddy/WAF 时设为 true）
	} `json:"server"`
	Database struct {
		Path string `json:"path"`
	} `json:"database"`
	JWT struct {
		Secret string `json:"secret"`
	} `json:"jwt"`
	Upload struct {
		Dir   string `json:"dir"`
		MaxMB int64  `json:"max_mb"`
	} `json:"upload"`
	Admin struct {
		Username   string `json:"username"`
		Password   string `json:"password"`    // 仅在管理员账号不存在时用于创建；已存在时需配合 force_reset 才会重置
		ForceReset bool   `json:"force_reset"` // 找回管理员密码用：为 true 时启动即用 password 重置，重置后请改回 false
	} `json:"admin"`
}

// Default 返回默认配置
func Default() *Config {
	c := &Config{}
	c.Server.Port = "8080"
	c.Database.Path = "data/ynxwxcb.db"
	c.JWT.Secret = "change-me-to-a-random-secret"
	c.Upload.Dir = "data/uploads"
	c.Upload.MaxMB = 50
	c.Admin.Username = "admin"
	c.Admin.Password = ""
	return c
}

// Load 从 JSON 文件加载配置，文件不存在时使用默认值
func Load(path string) (*Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
