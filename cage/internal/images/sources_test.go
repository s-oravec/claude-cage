package images

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseImages_ContainsExpected(t *testing.T) {
	sources := BaseImages()

	assert.Contains(t, sources, "ubuntu-24.04")
	assert.Contains(t, sources, "ubuntu-22.04")
	assert.Contains(t, sources, "debian-12")
}

func TestImageSource_HasRequiredFields(t *testing.T) {
	sources := BaseImages()
	ubuntu := sources["ubuntu-24.04"]

	assert.NotEmpty(t, ubuntu.URL)
	assert.NotEmpty(t, ubuntu.Name)
	assert.Contains(t, ubuntu.URL, "https://")
}

func TestListAvailable(t *testing.T) {
	names := ListAvailable()

	assert.Contains(t, names, "ubuntu-24.04")
	assert.Contains(t, names, "ubuntu-22.04")
	assert.Contains(t, names, "debian-12")
}

func TestGetSource_Exists(t *testing.T) {
	src, err := GetSource("ubuntu-24.04")

	assert.NoError(t, err)
	assert.Equal(t, "ubuntu-24.04", src.Name)
}

func TestGetSource_NotExists(t *testing.T) {
	_, err := GetSource("nonexistent")

	assert.Error(t, err)
}
