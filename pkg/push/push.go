package push

import (
	"fmt"
	"gitlab.com/sparetimecoders/build-tools/pkg/ci"
	"gitlab.com/sparetimecoders/build-tools/pkg/docker"
	"gitlab.com/sparetimecoders/build-tools/pkg/registry"
	"gitlab.com/sparetimecoders/build-tools/pkg/vcs"
	"os"
)

func Push(client docker.Client, dockerfile string) error {
	dir, _ := os.Getwd()
	currentVCS := vcs.Identify(dir)
	currentCI, err := ci.Identify(currentVCS)
	if err != nil {
		return err
	}
	currentRegistry := registry.Identify()
	if currentRegistry == nil {
		return fmt.Errorf("no Docker registry found")
	}

	if err := currentRegistry.Login(client); err != nil {
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
		if err := registry.PushImage(client, auth, tag); err != nil {
			return err
		}
	}
	return nil
}
