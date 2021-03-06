package clair

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/clair/api/v1"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/docker/reference"
	"github.com/jgsqware/clairctl/config"
	"github.com/jgsqware/clairctl/xstrings"
)

//Analyze return Clair Image analysis
func Analyze(image reference.Named, manifest distribution.Manifest) ImageAnalysis {
	switch manifest.(type) {
	case *schema1.SignedManifest:
		return Analyze(image, manifest.(schema1.SignedManifest))
	case *schema2.DeserializedManifest:
		logrus.Fatalf("Schema version 2 is not supported yet")

	}
	return ImageAnalysis{}
}

//V1Analyze return Clair Image analysis for Schema v1 image
func V1Analyze(image reference.Named, manifest schema1.SignedManifest) ImageAnalysis {
	c := len(manifest.FSLayers)
	res := []v1.LayerEnvelope{}

	for i := range manifest.FSLayers {
		blobsum := manifest.FSLayers[c-i-1].BlobSum.String()
		if config.IsLocal {
			blobsum = strings.TrimPrefix(blobsum, "sha256:")
		}
		lShort := xstrings.Substr(blobsum, 0, 12)

		if a, err := analyzeLayer(blobsum); err != nil {
			logrus.Infof("analysing layer [%v] %d/%d: %v", lShort, i+1, c, err)
		} else {
			logrus.Infof("analysing layer [%v] %d/%d", lShort, i+1, c)
			res = append(res, a)
		}
	}
	return ImageAnalysis{
		Registry:  xstrings.TrimPrefixSuffix(image.Hostname(), "http://", "/v2"),
		ImageName: manifest.Name,
		Tag:       manifest.Tag,
		Layers:    res,
	}
}

func analyzeLayer(id string) (v1.LayerEnvelope, error) {

	lURI := fmt.Sprintf("%v/layers/%v?vulnerabilities", uri, id)
	response, err := http.Get(lURI)
	if err != nil {
		return v1.LayerEnvelope{}, fmt.Errorf("analysing layer %v: %v", id, err)
	}
	defer response.Body.Close()

	var analysis v1.LayerEnvelope
	err = json.NewDecoder(response.Body).Decode(&analysis)
	if err != nil {
		return v1.LayerEnvelope{}, fmt.Errorf("reading layer analysis: %v", err)
	}
	if response.StatusCode != 200 {
		//TODO(jgsqware): should I show reponse body in case of error?
		return v1.LayerEnvelope{}, fmt.Errorf("receiving http error: %d", response.StatusCode)
	}

	return analysis, nil
}
