package config

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"
	"net/http"
)

type GithubVCS struct {
	git
	Token        string `yaml:"token" env:"GITHUB_TOKEN"`
	Organisation string `yaml:"organisation" env:"GITHUB_ORG"`
	Public       bool   `yaml:"public"`
	repoOwner    string
}

func (v *GithubVCS) Name() string {
	return "Github"
}

func (v *GithubVCS) Scaffold(name string) (*RepositoryInfo, error) {
	return v.scaffold(v.client().Repositories, name)
}

func (v *GithubVCS) Webhook(name, url string) error {
	return v.webhook(v.client().Repositories, name, url)
}

func (v *GithubVCS) Validate() error {
	if len(v.Token) == 0 {
		return errors.New("token is required")
	}
	return nil
}

var _ VCS = &GithubVCS{}

func (v *GithubVCS) client() *github.Client {
	client := github.NewClient(oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: v.Token},
	)))
	return client
}

func (v *GithubVCS) scaffold(repositoriesService RepositoriesService, name string) (*RepositoryInfo, error) {
	repo := &github.Repository{
		Name:     wrapString(name),
		Private:  wrapBool(v.Public),
		AutoInit: wrapBool(true),
	}
	repo, resp, err := repositoriesService.Create(context.Background(), v.Organisation, repo)
	if err != nil {
		return nil, err
	}

	v.repoOwner = v.Organisation
	if v.repoOwner == "" {
		v.repoOwner = *repo.Owner.Login
	}
	switch resp.StatusCode {
	case http.StatusCreated:
		preq := &github.ProtectionRequest{
			RequiredPullRequestReviews: &github.PullRequestReviewsEnforcementRequest{
				DismissStaleReviews:          true,
				RequiredApprovingReviewCount: 1,
			},
			EnforceAdmins: true,
		}

		_, response, err := repositoriesService.UpdateBranchProtection(context.Background(), v.repoOwner, *repo.Name, "master", preq)
		if err != nil || response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to set repository branch protection %s", response.Status)
		}
	default:
		return nil, fmt.Errorf("failed to create repository %s, %s", name, resp.Status)
	}
	return &RepositoryInfo{
		SSHURL:  *repo.SSHURL,
		HTTPURL: *repo.CloneURL,
	}, nil
}

func (v *GithubVCS) webhook(repositoriesService RepositoriesService, name, url string) error {
	hook := &github.Hook{
		Events: []string{
			"push",
			"pull_request",
			"deployment",
		},
		Config: map[string]interface{}{
			"url":          url,
			"content_type": "json",
		},
		Active: wrapBool(true),
	}

	_, resp, err := repositoriesService.CreateHook(context.Background(), v.repoOwner, name, hook)
	if err != nil || resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create webhook %s", resp.Status)
	}

	return nil
}

type RepositoriesService interface {
	Create(ctx context.Context, org string, repo *github.Repository) (*github.Repository, *github.Response, error)
	UpdateBranchProtection(ctx context.Context, owner, repo, branch string, preq *github.ProtectionRequest) (*github.Protection, *github.Response, error)
	CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, *github.Response, error)
}
