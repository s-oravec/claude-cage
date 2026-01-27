package images

import "errors"

// ImageSource defines a base image source
type ImageSource struct {
	Name        string
	URL         string
	Description string
}

// imageAliases maps short names to canonical image names
var imageAliases = map[string]string{
	// Default aliases point to latest stable/LTS
	"alpine":   "alpine-3.21",
	"ubuntu":   "ubuntu-24.04",
	"debian":   "debian-12",
	"rocky":    "rocky-9",
	"alma":     "alma-9",
	"fedora":   "fedora-41",
	"opensuse": "opensuse-15.6",
	"centos":   "centos-stream-9",
}

// BaseImages returns available base images
func BaseImages() map[string]ImageSource {
	return map[string]ImageSource{
		// Alpine Linux - minimal, fast boot (~250MB)
		"alpine-3.21": {
			Name:        "alpine-3.21",
			URL:         "https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/cloud/nocloud_alpine-3.21.2-x86_64-bios-cloudinit-r0.qcow2",
			Description: "Alpine Linux 3.21 (minimal, ~250MB)",
		},
		"alpine-3.20": {
			Name:        "alpine-3.20",
			URL:         "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/cloud/nocloud_alpine-3.20.6-x86_64-bios-cloudinit-r0.qcow2",
			Description: "Alpine Linux 3.20",
		},

		// Ubuntu LTS
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
		"ubuntu-20.04": {
			Name:        "ubuntu-20.04",
			URL:         "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img",
			Description: "Ubuntu 20.04 LTS (Focal Fossa)",
		},

		// Debian stable
		"debian-12": {
			Name:        "debian-12",
			URL:         "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2",
			Description: "Debian 12 (Bookworm)",
		},
		"debian-11": {
			Name:        "debian-11",
			URL:         "https://cloud.debian.org/images/cloud/bullseye/latest/debian-11-generic-amd64.qcow2",
			Description: "Debian 11 (Bullseye)",
		},

		// Rocky Linux (RHEL clone)
		"rocky-9": {
			Name:        "rocky-9",
			URL:         "https://dl.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud-Base.latest.x86_64.qcow2",
			Description: "Rocky Linux 9",
		},
		"rocky-8": {
			Name:        "rocky-8",
			URL:         "https://dl.rockylinux.org/pub/rocky/8/images/x86_64/Rocky-8-GenericCloud-Base.latest.x86_64.qcow2",
			Description: "Rocky Linux 8",
		},

		// AlmaLinux (RHEL clone)
		"alma-9": {
			Name:        "alma-9",
			URL:         "https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/AlmaLinux-9-GenericCloud-latest.x86_64.qcow2",
			Description: "AlmaLinux 9",
		},
		"alma-8": {
			Name:        "alma-8",
			URL:         "https://repo.almalinux.org/almalinux/8/cloud/x86_64/images/AlmaLinux-8-GenericCloud-latest.x86_64.qcow2",
			Description: "AlmaLinux 8",
		},

		// Fedora Server
		"fedora-41": {
			Name:        "fedora-41",
			URL:         "https://download.fedoraproject.org/pub/fedora/linux/releases/41/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-41-1.4.x86_64.qcow2",
			Description: "Fedora 41 Cloud",
		},
		"fedora-40": {
			Name:        "fedora-40",
			URL:         "https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2",
			Description: "Fedora 40 Cloud",
		},

		// openSUSE Leap
		"opensuse-15.6": {
			Name:        "opensuse-15.6",
			URL:         "https://download.opensuse.org/distribution/leap/15.6/appliances/openSUSE-Leap-15.6-Minimal-VM.x86_64-Cloud.qcow2",
			Description: "openSUSE Leap 15.6",
		},
		"opensuse-15.5": {
			Name:        "opensuse-15.5",
			URL:         "https://download.opensuse.org/distribution/leap/15.5/appliances/openSUSE-Leap-15.5-Minimal-VM.x86_64-Cloud.qcow2",
			Description: "openSUSE Leap 15.5",
		},

		// CentOS Stream
		"centos-stream-9": {
			Name:        "centos-stream-9",
			URL:         "https://cloud.centos.org/centos/9-stream/x86_64/images/CentOS-Stream-GenericCloud-9-latest.x86_64.qcow2",
			Description: "CentOS Stream 9",
		},
	}
}

// ResolveAlias resolves an image alias to canonical name
func ResolveAlias(name string) string {
	if canonical, ok := imageAliases[name]; ok {
		return canonical
	}
	return name
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

// GetSource returns an image source by name (supports aliases)
func GetSource(name string) (*ImageSource, error) {
	name = ResolveAlias(name)
	sources := BaseImages()
	src, ok := sources[name]
	if !ok {
		return nil, errors.New("unknown image: " + name)
	}
	return &src, nil
}
