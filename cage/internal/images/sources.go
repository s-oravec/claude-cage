package images

import "errors"

// ImageSource defines a base image source
type ImageSource struct {
	Name        string
	URL         string
	Description string
}

// BaseImages returns available base images
func BaseImages() map[string]ImageSource {
	return map[string]ImageSource{
		"ubuntu-24.04": {
			Name:        "ubuntu-24.04",
			URL:         "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			Description: "Ubuntu 24.04 LTS (Noble Numbat)",
		},
		"ubuntu-22.04": {
			Name:        "ubuntu-22.04",
			URL:         "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
			Description: "Ubuntu 22.04 LTS (Jammy Jellyfish)",
		},
		"debian-12": {
			Name:        "debian-12",
			URL:         "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2",
			Description: "Debian 12 (Bookworm)",
		},
		"alpine-3.20": {
			Name:        "alpine-3.20",
			URL:         "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/cloud/nocloud_alpine-3.20.6-x86_64-bios-cloudinit-r0.qcow2",
			Description: "Alpine Linux 3.20 (minimal, ~50MB)",
		},
	}
}

// ListAvailable returns names of available base images
func ListAvailable() []string {
	sources := BaseImages()
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	return names
}

// GetSource returns an image source by name
func GetSource(name string) (*ImageSource, error) {
	sources := BaseImages()
	src, ok := sources[name]
	if !ok {
		return nil, errors.New("unknown image: " + name)
	}
	return &src, nil
}
