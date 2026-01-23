# Fáza 11: Custom Images

Vytvorenie a správa custom images.

## Cieľ

- `cage image save` - uložiť bežiaci cage ako image
- `cage image list` - zoznam images
- `cage image delete` - zmazať image
- `cage image inspect` - detaily image

## Závisí na

- Fáza 10 (snapshots)

## Implementácia

### 1. Image types

```go
// internal/images/types.go
type Image struct {
    Name        string    `json:"name"`
    Type        string    `json:"type"` // base, custom
    Base        string    `json:"base"` // parent image (for custom)
    Size        int64     `json:"size_bytes"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
    Path        string    `json:"path"`
}
```

### 2. cage image save

```go
// internal/images/save.go
func Save(cageName, imageName, description string) error {
    // Validate cage exists and is running
    state := cage.LoadState(cageName)
    if state.Status != "running" {
        return errors.New("cage must be running to save")
    }

    // Check image name not taken
    if ImageExists(imageName) {
        return fmt.Errorf("image '%s' already exists", imageName)
    }

    // Get source disk path
    sourceDisk := filepath.Join(config.CagesDir(), cageName, "disk.qcow2")

    // Create new image (commit overlay to new file)
    destPath := filepath.Join(config.ImagesDir(), imageName+".qcow2")

    fmt.Println("Saving image (this may take a while)...")

    // Option 1: qemu-img convert (creates independent image)
    cmd := exec.Command("qemu-img", "convert",
        "-O", "qcow2",
        "-c", // compress
        sourceDisk,
        destPath)

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to save image: %w", err)
    }

    // Save metadata
    info, _ := os.Stat(destPath)
    img := Image{
        Name:        imageName,
        Type:        "custom",
        Base:        state.Image,
        Size:        info.Size(),
        Description: description,
        CreatedAt:   time.Now(),
        Path:        destPath,
    }
    saveImageMetadata(img)

    fmt.Printf("✓ Image '%s' saved (%.1f MB)\n",
        imageName, float64(info.Size())/1024/1024)

    return nil
}
```

### 3. cage image list

```go
// internal/images/list.go
func List() ([]Image, error) {
    var images []Image

    // List qcow2 files
    files, _ := filepath.Glob(filepath.Join(config.ImagesDir(), "*.qcow2"))

    for _, f := range files {
        name := strings.TrimSuffix(filepath.Base(f), ".qcow2")
        img := loadImageMetadata(name)
        if img == nil {
            // Base image without metadata
            info, _ := os.Stat(f)
            img = &Image{
                Name: name,
                Type: "base",
                Size: info.Size(),
                Path: f,
            }
        }
        images = append(images, *img)
    }

    return images, nil
}
```

### 4. cage image delete

```go
// internal/images/delete.go
func Delete(imageName string, force bool) error {
    img := loadImageMetadata(imageName)
    if img == nil {
        return ErrImageNotFound
    }

    // Check if base image
    if img.Type == "base" && !force {
        return errors.New("cannot delete base image, use --force")
    }

    // Check not in use
    cages, _ := cage.List()
    for _, c := range cages {
        if c.Image == imageName {
            return fmt.Errorf("image in use by cage '%s'", c.Name)
        }
    }

    // Delete file
    os.Remove(img.Path)

    // Delete metadata
    deleteImageMetadata(imageName)

    return nil
}
```

### 5. cage image inspect

```go
// internal/images/inspect.go
func Inspect(imageName string) (*ImageDetails, error) {
    img := loadImageMetadata(imageName)
    if img == nil {
        return nil, ErrImageNotFound
    }

    // Get qcow2 info
    cmd := exec.Command("qemu-img", "info", "--output=json", img.Path)
    out, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var qcowInfo struct {
        VirtualSize int64  `json:"virtual-size"`
        ActualSize  int64  `json:"actual-size"`
        Format      string `json:"format"`
        BackingFile string `json:"backing-filename"`
    }
    json.Unmarshal(out, &qcowInfo)

    return &ImageDetails{
        Image:       *img,
        VirtualSize: qcowInfo.VirtualSize,
        ActualSize:  qcowInfo.ActualSize,
        Format:      qcowInfo.Format,
        BackingFile: qcowInfo.BackingFile,
    }, nil
}
```

### 6. Image metadata storage

```
~/.claude-cage/images/
├── ubuntu-24.04.qcow2
├── debian-12.qcow2
├── my-custom.qcow2
└── metadata/
    ├── ubuntu-24.04.json
    ├── debian-12.json
    └── my-custom.json
```

### 7. CLI commands

```go
// cage image list
imageListCmd := &cobra.Command{
    Use:   "list",
    Short: "List available images",
    RunE: func(cmd *cobra.Command, args []string) error {
        images, err := images.List()
        if err != nil {
            return err
        }
        fmt.Println("NAME              TYPE    SIZE     CREATED")
        for _, img := range images {
            created := "-"
            if !img.CreatedAt.IsZero() {
                created = img.CreatedAt.Format("2006-01-02")
            }
            fmt.Printf("%-17s %-7s %-8s %s\n",
                img.Name, img.Type, formatSize(img.Size), created)
        }
        return nil
    },
}

// cage image save
imageSaveCmd := &cobra.Command{
    Use:   "save <cage-name>",
    Short: "Save cage as new image",
    RunE: func(cmd *cobra.Command, args []string) error {
        name, _ := cmd.Flags().GetString("name")
        desc, _ := cmd.Flags().GetString("description")
        return images.Save(args[0], name, desc)
    },
}
imageSaveCmd.Flags().StringP("name", "n", "", "Image name (required)")
imageSaveCmd.Flags().StringP("description", "d", "", "Description")
imageSaveCmd.MarkFlagRequired("name")

// cage image delete
// cage image inspect
```

## Acceptance test

```bash
# List base images
./cage image list
# NAME              TYPE    SIZE     CREATED
# ubuntu-24.04      base    285 MB   -
# debian-12         base    250 MB   -

# Create and customize a cage
./cage start --name setup
./cage ssh setup "sudo apt update && sudo apt install -y nodejs npm"
# (install stuff)

# Save as custom image
./cage image save setup --name nodejs-dev --description "Node.js development"
# Saving image (this may take a while)...
# ✓ Image 'nodejs-dev' saved (350 MB)

# List images
./cage image list
# NAME              TYPE    SIZE     CREATED
# ubuntu-24.04      base    285 MB   -
# debian-12         base    250 MB   -
# nodejs-dev        custom  350 MB   2024-01-23

# Inspect
./cage image inspect nodejs-dev
# Name:        nodejs-dev
# Type:        custom
# Base:        ubuntu-24.04
# Size:        350 MB
# Created:     2024-01-23 14:30:00
# Description: Node.js development

# Use custom image
./cage stop setup
./cage start --name project --image nodejs-dev
./cage ssh project "node --version"
# v20.x.x

# Delete custom image
./cage stop project
./cage image delete nodejs-dev
# ✓ Image deleted

# Cannot delete base without --force
./cage image delete ubuntu-24.04
# Error: cannot delete base image, use --force
```

## Deliverables

- [x] `cage image list`
- [x] `cage image save <cage> --name <image>`
- [x] `cage image save <cage> --name <image> --description <desc>`
- [x] `cage image delete <image>`
- [x] `cage image delete <image> --force`
- [x] `cage image inspect <image>`
- [x] Image metadata storage
- [x] qcow2 conversion and compression
- [x] In-use detection
- [x] Base image protection
