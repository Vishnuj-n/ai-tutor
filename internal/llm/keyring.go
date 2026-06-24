package llm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const keyringService = "ai-tutor"

func keyringUserForTier(tier string) string {
	tier = strings.TrimSpace(strings.ToLower(tier))
	if tier == "" {
		tier = "default"
	}
	return "llm-" + tier
}

func SaveAPIKey(tier string, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("api key is required")
	}
	return keyring.Set(keyringService, keyringUserForTier(tier), key)
}

func GetAPIKey(tier string) (string, error) {
	key, err := keyring.Get(keyringService, keyringUserForTier(tier))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(key), nil
}

func DeleteAPIKey(tier string) error {
	err := keyring.Delete(keyringService, keyringUserForTier(tier))
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

func HasAPIKey(tier string) bool {
	key, err := GetAPIKey(tier)
	return err == nil && key != ""
}

