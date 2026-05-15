package imgstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func ensureDir(p string) error { return os.MkdirAll(filepath.Dir(p), 0o755) }

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := ensureDir(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// CopyFromFile streams a file into the layer store at the named digest path,
// verifying the digest matches the file contents. Used for build flows that
// hand off a qcow2 file rather than bytes-in-memory.
func CopyFromFile(srcPath, digest string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst := LayerPath(digest)
	if err := ensureDir(dst); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".layer.*")
	if err != nil {
		return err
	}
	cleanup := func() { os.Remove(tmp.Name()) }

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), src); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	got := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if got != digest {
		tmp.Close()
		cleanup()
		return fmt.Errorf("digest mismatch: expected %s, computed %s", digest, got)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// PutLayerBytes writes a layer blob at its content-addressed path using an
// atomic temp-file + rename, with fsync before rename.
func PutLayerBytes(digest string, data []byte) error {
	return writeAtomic(LayerPath(digest), data, 0o644)
}

// GetLayerBytes reads the layer blob at the given digest.
func GetLayerBytes(digest string) ([]byte, error) {
	return os.ReadFile(LayerPath(digest))
}

// HasLayer reports whether a layer blob exists at the given digest.
func HasLayer(digest string) bool {
	_, err := os.Stat(LayerPath(digest))
	return err == nil
}

// PutManifestBytes writes a manifest blob at its content-addressed path using
// an atomic temp-file + rename, with fsync before rename.
func PutManifestBytes(digest string, data []byte) error {
	return writeAtomic(ManifestPath(digest), data, 0o644)
}

// GetManifestBytes reads the manifest blob at the given digest.
func GetManifestBytes(digest string) ([]byte, error) {
	return os.ReadFile(ManifestPath(digest))
}

// HasManifest reports whether a manifest blob exists at the given digest.
func HasManifest(digest string) bool {
	_, err := os.Stat(ManifestPath(digest))
	return err == nil
}

// WriteRef atomically writes the ref file for r to point at digest.
// The digest must be prefixed with "sha256:". Overwrites any existing ref.
func WriteRef(r Ref, digest string) error {
	if !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("ref digest must be sha256:")
	}
	return writeAtomic(r.RefPath(), []byte(digest+"\n"), 0o644)
}

// ReadRef returns the digest pointed to by r's ref file, trimming whitespace.
// Returns an os.IsNotExist error if the ref does not exist.
func ReadRef(r Ref) (string, error) {
	b, err := os.ReadFile(r.RefPath())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// DeleteRef removes the ref file for r. Returns nil if the ref does not exist.
func DeleteRef(r Ref) error {
	err := os.Remove(r.RefPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// HashFile computes sha256:<hex> of a file's contents via streaming.
// Used by build flows to digest layer qcow2 files before storing.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// MaterializeChain copies the top layer of a manifest to dstPath and rebases
// its backing-file pointer to baseImagePath. MVP single-layer flow only;
// multi-layer manifests return an error.
func MaterializeChain(manifestDigest string, baseImagePath, dstPath string) error {
	body, err := GetManifestBytes(manifestDigest)
	if err != nil {
		return err
	}
	var m manifestForMaterialize
	if err := json.Unmarshal(body, &m); err != nil {
		return err
	}
	if len(m.Layers) != 1 {
		return fmt.Errorf("multi-layer materialization not supported in MVP (got %d layers)", len(m.Layers))
	}
	src := LayerPath(m.Layers[0].Digest)
	if err := copyFileToDst(src, dstPath); err != nil {
		return err
	}
	cmd := exec.Command("qemu-img", "rebase", "-u", "-b", baseImagePath, "-F", "qcow2", dstPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("qemu-img rebase: %w: %s", err, string(out))
	}
	return nil
}

type manifestForMaterialize struct {
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

func copyFileToDst(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := ensureDir(dst); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
