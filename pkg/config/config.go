package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/apex/log"
	"github.com/caarlos0/env/v6"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"

	"github.com/buildtool/build-tools/pkg/ci"
	"github.com/buildtool/build-tools/pkg/registry"
	"github.com/buildtool/build-tools/pkg/vcs"
)

type Config struct {
	VCS                 *VCSConfig        `yaml:"vcs"`
	CI                  *CIConfig         `yaml:"ci"`
	Registry            *RegistryConfig   `yaml:"registry"`
	Targets             map[string]Target `yaml:"targets"`
	AvailableCI         []ci.CI
	AvailableRegistries []registry.Registry
}

type VCSConfig struct {
	VCS vcs.VCS
}

type CIConfig struct {
	Azure     *ci.Azure     `yaml:"azure"`
	Buildkite *ci.Buildkite `yaml:"buildkite"`
	Gitlab    *ci.Gitlab    `yaml:"gitlab"`
	Github    *ci.Github    `yaml:"github"`
	TeamCity  *ci.TeamCity  `yaml:"teamcity"`
}

type RegistryConfig struct {
	Dockerhub *registry.Dockerhub `yaml:"dockerhub"`
	ECR       *registry.ECR       `yaml:"ecr"`
	Github    *registry.Github    `yaml:"github"`
	Gitlab    *registry.Gitlab    `yaml:"gitlab"`
	Quay      *registry.Quay      `yaml:"quay"`
	GCR       *registry.GCR       `yaml:"gcr"`
}

type Target struct {
	Context    string `yaml:"context"`
	Namespace  string `yaml:"namespace,omitempty"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
}

const envBuildtoolsContent = "BUILDTOOLS_CONTENT"

func Load(dir string) (*Config, error) {
	cfg := InitEmptyConfig()

	if content, ok := os.LookupEnv(envBuildtoolsContent); ok {
		log.Debugf("Parsing config from env: %s\n", envBuildtoolsContent)
		if decoded, err := base64.StdEncoding.DecodeString(content); err != nil {
			log.Debugf("Failed to decode BASE64, falling back to plaintext\n")
			if err := parseConfig([]byte(content), cfg); err != nil {
				return cfg, err
			}
		} else {
			if err := parseConfig(decoded, cfg); err != nil {
				return cfg, err
			}
		}
	} else {
		err := parseConfigFiles(dir, func(dir string) error {
			return parseConfigFile(dir, cfg)
		})
		if err != nil {
			return cfg, err
		}
	}

	err := env.Parse(cfg)

	identifiedVcs := vcs.Identify(dir)
	cfg.VCS.VCS = identifiedVcs

	// TODO: Validate and clean config

	return cfg, err
}

func InitEmptyConfig() *Config {
	c := &Config{
		VCS: &VCSConfig{},
		CI: &CIConfig{
			Azure:     &ci.Azure{Common: &ci.Common{}},
			Buildkite: &ci.Buildkite{Common: &ci.Common{}},
			Gitlab:    &ci.Gitlab{Common: &ci.Common{}},
			Github:    &ci.Github{Common: &ci.Common{}},
			TeamCity:  &ci.TeamCity{Common: &ci.Common{}},
		},
		Registry: &RegistryConfig{
			Dockerhub: &registry.Dockerhub{},
			ECR:       &registry.ECR{},
			Github:    &registry.Github{},
			Gitlab:    &registry.Gitlab{},
			Quay:      &registry.Quay{},
			GCR:       &registry.GCR{},
		},
	}
	c.AvailableCI = []ci.CI{c.CI.Azure, c.CI.Buildkite, c.CI.Gitlab, c.CI.TeamCity, c.CI.Github}
	c.AvailableRegistries = []registry.Registry{c.Registry.Dockerhub, c.Registry.ECR, c.Registry.Github, c.Registry.Gitlab, c.Registry.Quay, c.Registry.GCR}
	return c
}

func (c *Config) CurrentVCS() vcs.VCS {
	return c.VCS.VCS
}

func (c *Config) CurrentCI() ci.CI {
	for _, x := range c.AvailableCI {
		if x.Configured() {
			x.SetVCS(c.CurrentVCS())
			return x
		}
	}
	x := &ci.No{Common: &ci.Common{}}
	x.SetVCS(c.CurrentVCS())
	return x
}

func (c *Config) CurrentRegistry() registry.Registry {
	for _, reg := range c.AvailableRegistries {
		if reg.Configured() {
			return reg
		}
	}
	return registry.NoDockerRegistry{}
}

func (c *Config) Print(target io.Writer) error {
	p := struct {
		CI       string            `yaml:"ci"`
		VCS      string            `yaml:"vcs"`
		Registry registry.Registry `yaml:"registry"`
		Targets  map[string]Target
	}{
		CI:       c.CurrentCI().Name(),
		VCS:      c.CurrentVCS().Name(),
		Registry: c.CurrentRegistry(),
		Targets:  c.Targets,
	}
	if out, err := yaml.Marshal(p); err != nil {
		return err
	} else {
		_, _ = target.Write(out)
	}
	return nil
}

func (c *Config) CurrentTarget(target string) (*Target, error) {
	if e, exists := c.Targets[target]; exists {
		return &e, nil
	}
	return nil, fmt.Errorf("no target matching %s found", target)
}

var abs = filepath.Abs

func parseConfigFiles(dir string, fn func(string) error) error {
	parent, err := abs(dir)
	if err != nil {
		return err
	}
	var files []string
	for !strings.HasSuffix(filepath.Clean(parent), string(os.PathSeparator)) {
		filename := filepath.Join(parent, ".buildtools.yaml")
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			files = append(files, filename)
		}

		parent = filepath.Dir(parent)
	}
	for i, file := range files {
		if i == 0 {
			log.Debugf("Parsing config from file: <green>'%s'</green>\n", file)
		} else {
			log.Debugf("Merging with config from file: <green>'%s'</green>\n", file)
		}
		if err := fn(file); err != nil {
			return err
		}
	}

	return nil
}

func parseConfigFile(filename string, cfg *Config) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return parseConfig(data, cfg)
}

func parseConfig(content []byte, config *Config) error {
	temp := &Config{}
	if err := UnmarshalStrict(content, temp); err != nil {
		return err
	} else {
		if err := mergo.Merge(config, temp); err != nil {
			return err
		}
		return validate(config)
	}
}

func UnmarshalStrict(content []byte, out interface{}) error {
	reader := bytes.NewReader(content)
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func validate(config *Config) error {
	elem := reflect.ValueOf(config.Registry).Elem()
	found := false
	for i := 0; i < elem.NumField(); i++ {
		f := elem.Field(i)
		if f.Interface().(registry.Registry).Configured() {
			if found {
				return fmt.Errorf("registry already defined, please check configuration")
			}
			found = true
		}
	}

	return nil
}
