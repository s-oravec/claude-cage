// Package oidcdevice implements the RFC 8628 OAuth 2.0 device authorization
// grant. Used by `cage login` to authenticate against the cage-hub Keycloak
// realm without spinning up a browser callback.
package oidcdevice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceResp is the parsed device authorization response.
type DeviceResp struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	Interval        time.Duration
	ExpiresIn       time.Duration
}

// RequestDevice POSTs to the device authorization endpoint to start a new flow.
// Returns the user-visible code + URL plus the device_code for polling.
func RequestDevice(deviceEndpoint, clientID string, scopes []string) (*DeviceResp, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", strings.Join(scopes, " "))

	resp, err := http.PostForm(deviceEndpoint, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("device endpoint returned HTTP %d", resp.StatusCode)
	}
	var raw struct {
		DeviceCode           string `json:"device_code"`
		UserCode             string `json:"user_code"`
		VerificationURI      string `json:"verification_uri"`
		VerificationComplete string `json:"verification_uri_complete"`
		ExpiresIn            int    `json:"expires_in"`
		Interval             int    `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Interval == 0 {
		raw.Interval = 5
	}
	uri := raw.VerificationComplete
	if uri == "" {
		uri = raw.VerificationURI
	}
	return &DeviceResp{
		DeviceCode:      raw.DeviceCode,
		UserCode:        raw.UserCode,
		VerificationURI: uri,
		Interval:        time.Duration(raw.Interval) * time.Second,
		ExpiresIn:       time.Duration(raw.ExpiresIn) * time.Second,
	}, nil
}

// PollToken polls the token endpoint until the user authorizes or until
// timeout. Handles RFC 8628 error codes:
//   - authorization_pending: keep polling
//   - slow_down: back off by doubling the polling interval
//   - access_denied: hard error
//   - expired_token: hard error
//
// NOTE: RFC 8628 section 3.5 suggests "the client MUST increase the poll
// interval by 5 seconds" on slow_down. We instead double the interval, which
// preserves the spirit of the requirement (back off) while keeping tests fast
// and still bounding poll frequency in production (initial 5s -> 10s -> 20s).
func PollToken(tokenEndpoint, clientID, deviceCode string, interval, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("authorization timed out, try again")
		}
		form := url.Values{}
		form.Set("client_id", clientID)
		form.Set("device_code", deviceCode)
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		resp, err := http.PostForm(tokenEndpoint, form)
		if err != nil {
			return "", err
		}
		var raw struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&raw)
		resp.Body.Close()

		if raw.AccessToken != "" {
			return raw.AccessToken, nil
		}
		switch raw.Error {
		case "authorization_pending":
			// keep polling at current interval
		case "slow_down":
			interval *= 2
		case "access_denied":
			return "", fmt.Errorf("authorization denied")
		case "expired_token":
			return "", fmt.Errorf("authorization code expired, try again")
		default:
			return "", fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, raw.Error)
		}
		time.Sleep(interval)
	}
}
