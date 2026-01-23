package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestListCmd_Exists(t *testing.T) {
	cmd := NewListCmd()

	assert.Equal(t, "list", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestListCmd_HasJsonFlag(t *testing.T) {
	cmd := NewListCmd()

	flag := cmd.Flag("json")
	assert.NotNil(t, flag)
}

func TestListCmd_HasAlias(t *testing.T) {
	cmd := NewListCmd()

	assert.Contains(t, cmd.Aliases, "ls")
}

func TestFormatUptime_Zero(t *testing.T) {
	result := formatUptime(time.Time{})
	assert.Equal(t, "-", result)
}

func TestFormatUptime_Seconds(t *testing.T) {
	start := time.Now().Add(-30 * time.Second)
	result := formatUptime(start)
	assert.Contains(t, result, "s")
	assert.NotContains(t, result, "m")
}

func TestFormatUptime_Minutes(t *testing.T) {
	start := time.Now().Add(-45 * time.Minute)
	result := formatUptime(start)
	assert.Contains(t, result, "m")
	assert.NotContains(t, result, "h")
}

func TestFormatUptime_Hours(t *testing.T) {
	start := time.Now().Add(-5 * time.Hour)
	result := formatUptime(start)
	assert.Contains(t, result, "h")
}

func TestFormatUptime_Days(t *testing.T) {
	start := time.Now().Add(-48 * time.Hour)
	result := formatUptime(start)
	assert.Contains(t, result, "d")
}
