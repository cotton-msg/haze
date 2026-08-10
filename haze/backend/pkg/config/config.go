package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Config читает YAML-файл и предоставляет доступ к значениям по точечному пути.
// Переменные окружения имеют приоритет над YAML (см. EnvStr/EnvInt/...).
type Config struct {
	data map[string]interface{}
}

// Load читает YAML-файл. Если файла нет — возвращает пустую конфигурацию без ошибки.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{data: map[string]interface{}{}}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &Config{data: m}, nil
}

// Get возвращает значение по точечному пути, например "server.port".
func (c *Config) Get(path string) interface{} {
	var cur interface{} = c.data
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

// EnvStr возвращает значение env-переменной, иначе из YAML, иначе default.
func (c *Config) EnvStr(envKey, yamlPath, def string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if v, ok := c.Get(yamlPath).(string); ok && v != "" {
		return v
	}
	return def
}

// EnvInt возвращает int-значение (env > YAML > default).
func (c *Config) EnvInt(envKey, yamlPath string, def int) int {
	if v := os.Getenv(envKey); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	switch v := c.Get(yamlPath).(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// EnvBool возвращает bool-значение (env > YAML > default).
func (c *Config) EnvBool(envKey, yamlPath string, def bool) bool {
	if v := os.Getenv(envKey); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	switch v := c.Get(yamlPath).(type) {
	case bool:
		return v
	case string:
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// EnvDuration возвращает duration-значение (env > YAML > default).
func (c *Config) EnvDuration(envKey, yamlPath string, def time.Duration) time.Duration {
	if v := os.Getenv(envKey); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	if v, ok := c.Get(yamlPath).(string); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// Str возвращает строку из YAML без env-override.
func (c *Config) Str(yamlPath, def string) string {
	if v, ok := c.Get(yamlPath).(string); ok && v != "" {
		return v
	}
	return def
}

// Int возвращает int из YAML без env-override.
func (c *Config) Int(yamlPath string, def int) int {
	switch v := c.Get(yamlPath).(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// EnvStrSlice возвращает список строк: env (через запятую) > YAML-массив > default.
func (c *Config) EnvStrSlice(envKey, yamlPath string, def []string) []string {
	if v := os.Getenv(envKey); v != "" {
		var out []string
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if list, ok := c.Get(yamlPath).([]interface{}); ok {
		var out []string
		for _, item := range list {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return def
}
