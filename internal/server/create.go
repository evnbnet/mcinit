package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/download"
	"github.com/evanb/mcinit/internal/javart"
	"github.com/evanb/mcinit/internal/provider"
)

type Plan struct {
	Config   config.Config
	Dir      string
	Artifact provider.Artifact
	Java     javart.Runtime
}

type Result struct {
	Dir       string
	JarPath   string
	StartPath string
}

func CheckTarget(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("the folder %s already exists and is not empty", dir)
	}
	return nil
}

func Create(ctx context.Context, plan Plan) (result Result, err error) {
	if !plan.Config.AcceptedEULA {

		return Result{}, fmt.Errorf("the Minecraft EULA was not accepted")
	}

	preexisting := true
	if _, statErr := os.Stat(plan.Dir); os.IsNotExist(statErr) {
		preexisting = false
		if err := os.MkdirAll(plan.Dir, 0o755); err != nil {
			return Result{}, fmt.Errorf("could not create %s: %w", plan.Dir, err)
		}
	}
	defer func() {
		if err != nil && !preexisting {
			os.RemoveAll(plan.Dir)
		}
	}()

	jar := filepath.Join(plan.Dir, plan.Artifact.Filename)
	err = download.Fetch(ctx, download.Request{
		URL:      plan.Artifact.URL,
		Dest:     jar,
		Size:     plan.Artifact.Size,
		Algo:     plan.Artifact.Algo,
		Checksum: plan.Artifact.Checksum,
		Label:    plan.Artifact.Filename,
	})
	if err != nil {
		return Result{}, err
	}

	data := newTemplateData(plan.Config, plan.Artifact, plan.Java.Path, plan.Java.Major)
	if err = renderAll(plan.Dir, data); err != nil {
		return Result{}, err
	}

	return Result{
		Dir:       plan.Dir,
		JarPath:   jar,
		StartPath: filepath.Join(plan.Dir, "start.bat"),
	}, nil
}
