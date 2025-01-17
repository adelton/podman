package machine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"

	"github.com/blang/semver/v4"
	"github.com/containers/podman/v4/version"
)

// quay.io/libpod/podman-machine-images:4.6

const (
	diskImages = "podman-machine-images"
	registry   = "quay.io"
	repo       = "libpod"
)

type OSVersion struct {
	*semver.Version
}

type Disker interface {
	Pull() error
	Decompress(compressedFile *VMFile) (*VMFile, error)
	DiskEndpoint() string
	Unpack() (*VMFile, error)
}

type OCIOpts struct {
	Scheme *OCIKind
	Dir    *string
}

type OCIKind string

var (
	OCIDir      OCIKind = "oci-dir"
	OCIRegistry OCIKind = "docker"
	OCIUnknown  OCIKind = "unknown"
)

func (o OCIKind) String() string {
	switch o {
	case OCIDir:
		return string(OCIDir)
	case OCIRegistry:
		return string(OCIRegistry)
	}
	return string(OCIUnknown)
}

func (o OCIKind) IsOCIDir() bool {
	return o == OCIDir
}

func StripOCIReference(input string) string {
	return strings.TrimPrefix(input, "docker://")
}

func getVersion() *OSVersion {
	v := version.Version

	// OVERRIDES FOR DEV ONLY
	v.Minor = 6
	v.Pre = nil
	// OVERRIDES FOR DEV ONLY

	return &OSVersion{&v}
}

func (o *OSVersion) majorMinor() string {
	return fmt.Sprintf("%d.%d", o.Major, o.Minor)
}

func (o *OSVersion) diskImage(diskFlavor ImageFormat) string {
	return fmt.Sprintf("%s/%s/%s:%s-%s", registry, repo, diskImages, o.majorMinor(), diskFlavor.string())
}

func unpackOCIDir(ociTb, machineImageDir string) (*VMFile, error) {
	imageFileName, err := findTarComponent(ociTb)
	if err != nil {
		return nil, err
	}

	unpackedFileName := filepath.Join(machineImageDir, imageFileName)

	f, err := os.Open(ociTb)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	uncompressedReader, _, err := compression.AutoDecompress(f)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := uncompressedReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	logrus.Debugf("untarring %q to %q", ociTb, machineImageDir)
	if err := archive.Untar(uncompressedReader, machineImageDir, &archive.TarOptions{
		NoLchown: true,
	}); err != nil {
		return nil, err
	}

	return NewMachineFile(unpackedFileName, nil)
}

func localOCIDiskImageDir(blobDirPath string, localBlob *types.BlobInfo) string {
	return filepath.Join(blobDirPath, "blobs", "sha256", localBlob.Digest.Hex())
}

func finalFQImagePathName(vmName, imageName string) string {
	// imageName here is fully qualified. we need to break
	// it apart and add the vmname
	baseDir, filename := filepath.Split(imageName)
	return filepath.Join(baseDir, fmt.Sprintf("%s-%s", vmName, filename))
}
