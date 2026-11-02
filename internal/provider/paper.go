package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/mcver"
)

var paperAPI = "https://fill.papermc.io/v3/projects/paper"

const paperDownloadKey = "server:default"

type paper struct{}

func (paper) Software() config.Software { return config.Paper }

type paperVersions struct {
	Versions map[string][]string `json:"versions"`
}

func (p paper) Versions(ctx context.Context) ([]mcver.Version, error) {
	var body paperVersions
	if err := getJSON(ctx, paperAPI, &body); err != nil {
		return nil, err
	}
	var all []string
	for _, group := range body.Versions {
		all = append(all, group...)
	}
	return sortNewestFirst(all), nil
}

type paperBuild struct {
	ID        int    `json:"id"`
	Channel   string `json:"channel"`
	Downloads map[string]struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		Size      int64  `json:"size"`
		Checksums struct {
			SHA256 string `json:"sha256"`
		} `json:"checksums"`
	} `json:"downloads"`
}

func (p paper) Resolve(ctx context.Context, version string) (Artifact, error) {
	url := fmt.Sprintf("%s/versions/%s/builds/latest", paperAPI, version)

	var build paperBuild
	if err := getJSON(ctx, url, &build); err != nil {
		if errors.Is(err, errNotFound) {
			return Artifact{}, fmt.Errorf("Paper has no version %s", version)
		}
		return Artifact{}, err
	}

	dl, ok := build.Downloads[paperDownloadKey]
	if !ok {
		return Artifact{}, fmt.Errorf("Paper %s build %d has no server jar", version, build.ID)
	}
	if dl.URL == "" || dl.Checksums.SHA256 == "" {
		return Artifact{}, fmt.Errorf("Paper %s returned a build with no URL or no checksum", version)
	}

	return Artifact{
		Filename: dl.Name,
		URL:      dl.URL,
		Size:     dl.Size,
		Build:    strconv.Itoa(build.ID),
		Channel:  build.Channel,
		Algo:     SHA256,
		Checksum: dl.Checksums.SHA256,
	}, nil
}
