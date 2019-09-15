package push

import (
	"gitlab.com/sparetimecoders/build-tools/pkg/config"
	"gitlab.com/sparetimecoders/build-tools/pkg/docker"
	"io"
	"os"
)

func Push(client docker.Client, dockerfile string, out, eout io.Writer) error {
	dir, _ := os.Getwd()
	cfg, err := config.Load(dir, out)
	if err != nil {
		return err
	}
	currentCI, err := cfg.CurrentCI()
	if err != nil {
		return err
	}
	currentRegistry, err := cfg.CurrentRegistry()
	if err != nil {
		return err
	}

	if err := currentRegistry.Login(client, out); err != nil {
		return err
	}

	auth := currentRegistry.GetAuthInfo()

	if err := currentRegistry.Create(currentCI.BuildName()); err != nil {
		return err
	}

	// TODO: Parse Dockerfile and push each stage for caching?

	tags := []string{
		docker.Tag(currentRegistry.RegistryUrl(), currentCI.BuildName(), currentCI.Commit()),
		docker.Tag(currentRegistry.RegistryUrl(), currentCI.BuildName(), currentCI.BranchReplaceSlash()),
	}
	if currentCI.Branch() == "master" {
		tags = append(tags, docker.Tag(currentRegistry.RegistryUrl(), currentCI.BuildName(), "latest"))
	}

	for _, tag := range tags {
		if err := currentRegistry.PushImage(client, auth, tag, out, eout); err != nil {
			return err
		}
	}
	return nil
}
