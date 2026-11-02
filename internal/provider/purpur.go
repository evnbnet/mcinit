package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/mcver"
)

var purpurAPI = "https://api.purpurmc.org/v2/purpur"

type purpur struct{}

func (purpur) Software() config.Software { return config.Purpur }

type purpurVersions struct {
	Versions []string `json:"versions"`
}

func (p purpur) Versions(ctx context.Context) ([]mcver.Version, error) {
	var body purpurVersions
	if err := getJSON(ctx, purpurAPI, &body); err != nil {
		return nil, err
	}
	return sortNewestFirst(body.Versions), nil
}

type purpurBuild struct {
	Version string `json:"version"`
	Build   string `json:"build"`
	Result  string `json:"result"`
	MD5     string `json:"md5"`
}

func (p purpur) Resolve(ctx context.Context, version string) (Artifact, error) {
	var build purpurBuild
	if err := getJSON(ctx, fmt.Sprintf("%s/%s/latest", purpurAPI, version), &build); err != nil {
		if errors.Is(err, errNotFound) {
			return Artifact{}, fmt.Errorf("Purpur has no version %s", version)
		}
		return Artifact{}, err
	}
	if build.Result != "" && build.Result != "SUCCESS" {
		return Artifact{}, fmt.Errorf("the latest Purpur %s build failed (%s); try another version", version, build.Result)
	}
	if build.Build == "" || build.MD5 == "" {
		return Artifact{}, fmt.Errorf("Purpur %s returned a build with no number or no checksum", version)
	}

	return Artifact{
		Filename: fmt.Sprintf("purpur-%s-%s.jar", version, build.Build),
		URL:      fmt.Sprintf("%s/%s/%s/download", purpurAPI, version, build.Build),
		Build:    build.Build,
		Algo:     MD5,
		Checksum: build.MD5,
	}, nil
}
