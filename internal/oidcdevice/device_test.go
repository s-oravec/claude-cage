package oidcdevice

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDevice_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "client_id=cage-cli&scope=openid+profile", readPostForm(r))
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":      "dc",
			"user_code":        "ABCD-1234",
			"verification_uri": "https://kc/device",
			"expires_in":       600,
			"interval":         5,
		})
	}))
	defer srv.Close()

	got, err := RequestDevice(srv.URL, "cage-cli", []string{"openid", "profile"})
	require.NoError(t, err)
	assert.Equal(t, "dc", got.DeviceCode)
	assert.Equal(t, "ABCD-1234", got.UserCode)
	assert.Equal(t, time.Duration(5)*time.Second, got.Interval)
}

func TestPollToken_AuthPending_ThenSuccess(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"authorization_pending"}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"access_token":"ey...","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	tok, err := PollToken(srv.URL, "cage-cli", "dc", 10*time.Millisecond, time.Second)
	require.NoError(t, err)
	assert.Equal(t, "ey...", tok.AccessToken)
	assert.Equal(t, time.Hour, tok.ExpiresIn)
	assert.GreaterOrEqual(t, call, 2)
}

func TestRefresh_PostsRefreshGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.PostForm.Get("grant_type"))
		assert.Equal(t, "cage-cli", r.PostForm.Get("client_id"))
		assert.Equal(t, "old-refresh", r.PostForm.Get("refresh_token"))
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-acc",
			"refresh_token": "new-ref",
			"expires_in":    300,
		})
	}))
	defer srv.Close()

	got, err := Refresh(srv.URL, "cage-cli", "old-refresh")
	require.NoError(t, err)
	assert.Equal(t, "new-acc", got.AccessToken)
	assert.Equal(t, "new-ref", got.RefreshToken)
	assert.Equal(t, 300*time.Second, got.ExpiresIn)
}

func TestRefresh_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	_, err := Refresh(srv.URL, "cage-cli", "bad")
	require.Error(t, err)
}

func TestPollToken_SlowDown_BacksOff(t *testing.T) {
	call := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		if call <= 2 {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"slow_down"}`))
			return
		}
		w.Write([]byte(`{"access_token":"t"}`))
	}))
	defer srv.Close()
	_, err := PollToken(srv.URL, "cage-cli", "dc", 5*time.Millisecond, time.Second)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, call, 3)
}

func TestPollToken_TimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	defer srv.Close()
	_, err := PollToken(srv.URL, "cage-cli", "dc", 5*time.Millisecond, 50*time.Millisecond)
	require.Error(t, err)
}

func readPostForm(r *http.Request) string {
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.PostForm.Encode()
}
