package api

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pulsewatch/pulsewatch/backend/internal/models"
	"github.com/pulsewatch/pulsewatch/backend/internal/monitor"
)

func validateCreate(req models.CreateMonitorRequest) (name, url string, interval int, enabled bool, err error) {
	name = strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", 0, false, fmt.Errorf("name is required")
	}
	if utf8.RuneCountInString(name) > 120 {
		return "", "", 0, false, fmt.Errorf("name must be at most 120 characters")
	}

	url = strings.TrimSpace(req.URL)
	if _, err := monitor.ValidatePublicURL(url); err != nil {
		return "", "", 0, false, err
	}

	interval = monitor.DefaultInterval
	if req.IntervalSeconds != nil {
		interval = *req.IntervalSeconds
	}
	if interval < monitor.MinInterval || interval > monitor.MaxInterval {
		return "", "", 0, false, fmt.Errorf("interval_seconds must be between %d and %d", monitor.MinInterval, monitor.MaxInterval)
	}

	enabled = true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return name, url, interval, enabled, nil
}

func validateUpdate(req models.UpdateMonitorRequest) error {
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return fmt.Errorf("name cannot be empty")
		}
		if utf8.RuneCountInString(n) > 120 {
			return fmt.Errorf("name must be at most 120 characters")
		}
		*req.Name = n
	}
	if req.URL != nil {
		u := strings.TrimSpace(*req.URL)
		if _, err := monitor.ValidatePublicURL(u); err != nil {
			return err
		}
		*req.URL = u
	}
	if req.IntervalSeconds != nil {
		if *req.IntervalSeconds < monitor.MinInterval || *req.IntervalSeconds > monitor.MaxInterval {
			return fmt.Errorf("interval_seconds must be between %d and %d", monitor.MinInterval, monitor.MaxInterval)
		}
	}
	return nil
}
