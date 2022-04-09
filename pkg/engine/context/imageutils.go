package context

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/distribution/distribution/reference"
	"github.com/go-logr/logr"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ImageInfo struct {
	// Registry is the URL address of the image registry e.g. `docker.io`
	Registry string `json:"registry,omitempty"`

	// Name is the image name portion e.g. `busybox`
	Name string `json:"name"`

	// Path is the repository path and image name e.g. `some-repository/busybox`
	Path string `json:"path"`

	// Tag is the image tag e.g. `v2`
	Tag string `json:"tag,omitempty"`

	// Digest is the image digest portion e.g. `sha256:128c6e3534b842a2eec139999b8ce8aa9a2af9907e2b9269550809d18cd832a3`
	Digest string `json:"digest,omitempty"`

	// JSONPointer is full JSON path to this image e.g. `/spec/containers/0/image`
	JSONPointer string `json:"jsonPath,omitempty"`
}

func (i *ImageInfo) String() string {
	image := i.Registry + "/" + i.Path + ":" + i.Tag
	// image that needs only digest and not the tag
	if i.Digest != "" {
		image = i.Registry + "/" + i.Path + "@" + i.Digest
	}
	return image
}

type ContainerImage struct {
	ImageInfo
	Name string
}

type Images struct {
	InitContainers      map[string]ImageInfo `json:"initContainers,omitempty"`
	Containers          map[string]ImageInfo `json:"containers"`
	EphemeralContainers map[string]ImageInfo `json:"ephemeralContainers"`
}

func newImages(initContainersImgs, containersImgs, ephemeralContainersImgs []ContainerImage) Images {
	initContainers := make(map[string]ImageInfo)
	for _, resource := range initContainersImgs {
		initContainers[resource.Name] = resource.ImageInfo
	}
	containers := make(map[string]ImageInfo)
	for _, resource := range containersImgs {
		containers[resource.Name] = resource.ImageInfo
	}
	ephemeralContainers := make(map[string]ImageInfo)
	for _, resource := range ephemeralContainersImgs {
		ephemeralContainers[resource.Name] = resource.ImageInfo
	}
	return Images{
		InitContainers:      initContainers,
		Containers:          containers,
		EphemeralContainers: ephemeralContainers,
	}
}

type imageExtractor struct {
	fields []string
}

func (i imageExtractor) extract(tag string, resource *unstructured.Unstructured) []ContainerImage {
	f := append(i.fields[:len(i.fields):len(i.fields)], tag)
	if containers, ok, _ := unstructured.NestedSlice(resource.UnstructuredContent(), f...); ok {
		return extractImageInfos(containers, "/"+strings.Join(f, "/"))
	}
	return nil
}

var extractors = map[string]imageExtractor{
	"Pod":         {[]string{"spec"}},
	"CronJob":     {[]string{"spec", "jobTemplate", "spec", "template", "spec"}},
	"Deployment":  {[]string{"spec", "template", "spec"}},
	"DaemonSet":   {[]string{"spec", "template", "spec"}},
	"Job":         {[]string{"spec", "template", "spec"}},
	"StatefulSet": {[]string{"spec", "template", "spec"}},
}

func extractImageInfo(resource *unstructured.Unstructured, log logr.Logger) (initContainersImgs, containersImgs, ephemeralContainersImgs []ContainerImage) {
	extractor := extractors[resource.GetKind()]
	initContainersImgs = extractor.extract("initContainers", resource)
	containersImgs = extractor.extract("containers", resource)
	ephemeralContainersImgs = extractor.extract("ephemeralContainers", resource)
	return
}

func extractImageInfos(containers []interface{}, jsonPath string) []ContainerImage {
	img, err := convertToImageInfo(containers, jsonPath)
	if err != nil {
		logger.Error(err, "failed to extract image info", "element", containers)
	}
	return img
}

func convertToImageInfo(containers []interface{}, jsonPath string) (images []ContainerImage, err error) {
	var errs []string
	var index = 0
	for _, ctr := range containers {
		if container, ok := ctr.(map[string]interface{}); ok {
			var name, image string
			name = container["name"].(string)
			if _, ok := container["image"]; ok {
				image = container["image"].(string)
			}
			jp := strings.Join([]string{jsonPath, strconv.Itoa(index), "image"}, "/")
			imageInfo, err := newImageInfo(image, jp)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			images = append(images, ContainerImage{*imageInfo, name})
		}
		index++
	}
	if len(errs) == 0 {
		return images, nil
	}
	return images, errors.Errorf("%s", strings.Join(errs, ";"))
}

func newImageInfo(image, jsonPointer string) (*ImageInfo, error) {
	image = addDefaultDomain(image)
	ref, err := reference.Parse(image)
	if err != nil {
		return nil, errors.Wrapf(err, "bad image: %s", image)
	}
	var registry, path, name, tag, digest string
	if named, ok := ref.(reference.Named); ok {
		registry = reference.Domain(named)
		path = reference.Path(named)
		name = path[strings.LastIndex(path, "/")+1:]
	}
	if tagged, ok := ref.(reference.Tagged); ok {
		tag = tagged.Tag()
	}
	if digested, ok := ref.(reference.Digested); ok {
		digest = digested.Digest().String()
	}
	// set default tag - the domain is set via addDefaultDomain before parsing
	if digest == "" && tag == "" {
		tag = "latest"
	}
	return &ImageInfo{
		Registry:    registry,
		Name:        name,
		Path:        path,
		Tag:         tag,
		Digest:      digest,
		JSONPointer: jsonPointer,
	}, nil
}

func addDefaultDomain(name string) string {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost" && strings.ToLower(name[:i]) == name[:i]) {
		return "docker.io/" + name
	}
	return name
}

// MutateResourceWithImageInfo will set images to their canonical form so that they can be compared
// in a predictable manner. This sets the default registry as `docker.io` and the tag as `latest` if
// these are missing.
func MutateResourceWithImageInfo(raw []byte, ctx Interface) error {
	images := ctx.ImageInfo()
	if images == nil {
		return nil
	}
	buildJSONPatch := func(op, path, value string) []byte {
		p := fmt.Sprintf(`{ "op": "%s", "path": "%s", "value":"%s" }`, op, path, value)
		return []byte(p)
	}
	var patches [][]byte
	for _, info := range images.Containers {
		patches = append(patches, buildJSONPatch("replace", info.JSONPointer, info.String()))
	}
	for _, info := range images.InitContainers {
		patches = append(patches, buildJSONPatch("replace", info.JSONPointer, info.String()))
	}
	for _, info := range images.EphemeralContainers {
		patches = append(patches, buildJSONPatch("replace", info.JSONPointer, info.String()))
	}
	patchedResource, err := engineutils.ApplyPatches(raw, patches)
	if err != nil {
		return err
	}
	return AddResource(ctx, patchedResource)
}
