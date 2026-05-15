package manifest

const (
	MediaTypeManifestV1 = "application/vnd.cage.manifest.v1+json"
	MediaTypeLayerV1    = "application/vnd.cage.layer.v1.qcow2"
	SchemaVersionV1     = 1
)

type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	MediaType     string  `json:"mediaType"`
	Base          Base    `json:"base"`
	Layers        []Layer `json:"layers"`
	Config        Config  `json:"config"`
}

type Base struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type Layer struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

type Config struct {
	OS          string     `json:"os"`
	Arch        string     `json:"arch"`
	User        string     `json:"user,omitempty"`
	Workdir     string     `json:"workdir,omitempty"`
	Env         []string   `json:"env,omitempty"`
	Description string     `json:"description,omitempty"`
	Readme      string     `json:"readme,omitempty"`
	Cagefile    string     `json:"cagefile,omitempty"`
	Resources   *Resources `json:"resources,omitempty"`
}

type Resources struct {
	MemoryMB int `json:"memory_mb,omitempty"`
	VCPU     int `json:"vcpu,omitempty"`
	DiskGB   int `json:"disk_gb,omitempty"`
}
