package docker_registry

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/flant/werf/pkg/image"
)

const DockerHubImplementationName = "dockerhub"

type DockerHubUnauthorizedError apiError

type DockerHubNotFoundError apiError

var dockerHubPatterns = []string{"^index\\.docker\\.io", "^registry\\.hub\\.docker\\.com"}

type dockerHub struct {
	*defaultImplementation
	dockerHubApi
	dockerHubCredentials
}

type dockerHubOptions struct {
	defaultImplementationOptions
	dockerHubCredentials
}

type dockerHubCredentials struct {
	username string
	password string
	token    string
}

func newDockerHub(options dockerHubOptions) (*dockerHub, error) {
	d, err := newDefaultImplementation(options.defaultImplementationOptions)
	if err != nil {
		return nil, err
	}

	dockerHub := &dockerHub{
		defaultImplementation: d,
		dockerHubApi:          newDockerHubApi(),
		dockerHubCredentials:  options.dockerHubCredentials,
	}

	return dockerHub, nil
}

func (r *dockerHub) DeleteRepo(reference string) error {
	return r.deleteRepo(reference)
}

func (r *dockerHub) DeleteRepoImage(repoImageList ...*image.Info) error {
	for _, repoImage := range repoImageList {
		if err := r.deleteRepoImage(repoImage); err != nil {
			return err
		}
	}

	return nil
}

func (r *dockerHub) deleteRepo(reference string) error {
	token, err := r.getToken()
	if err != nil {
		return err
	}

	account, project, err := r.parseReference(reference)
	if err != nil {
		return err
	}

	resp, err := r.dockerHubApi.deleteRepository(account, project, token)
	if resp != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return DockerHubUnauthorizedError{error: err}
		} else if resp.StatusCode == http.StatusNotFound {
			return DockerHubNotFoundError{error: err}
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func (r *dockerHub) deleteRepoImage(repoImage *image.Info) error {
	token, err := r.getToken()
	if err != nil {
		return err
	}

	account, project, err := r.parseReference(repoImage.Repository)
	if err != nil {
		return err
	}

	resp, err := r.dockerHubApi.deleteTag(account, project, repoImage.Tag, token)
	if resp != nil {
		if resp.StatusCode == http.StatusUnauthorized {
			return DockerHubUnauthorizedError{error: err}
		} else if resp.StatusCode == http.StatusNotFound {
			return DockerHubNotFoundError{error: err}
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func (r *dockerHub) getToken() (string, error) {
	if r.dockerHubCredentials.token == "" {
		token, resp, err := r.dockerHubApi.getToken(r.dockerHubCredentials.username, r.dockerHubCredentials.password)
		if resp != nil {
			if resp.StatusCode == http.StatusUnauthorized {
				return "", DockerHubUnauthorizedError{error: err}
			} else if resp.StatusCode == http.StatusNotFound {
				return "", DockerHubNotFoundError{error: err}
			}
		}

		if err != nil {
			return "", err
		}

		r.dockerHubCredentials.token = token
	}

	return r.dockerHubCredentials.token, nil
}

func (r *dockerHub) String() string {
	return DockerHubImplementationName
}

func (r *dockerHub) parseReference(reference string) (string, string, error) {
	parsedReference, err := name.NewTag(reference)
	if err != nil {
		return "", "", err
	}

	repositoryParts := strings.Split(parsedReference.RepositoryStr(), "/")

	var account, project string
	if len(repositoryParts) == 2 {
		account = repositoryParts[0]
		project = repositoryParts[1]
	} else {
		return "", "", fmt.Errorf("unexpected reference %s", reference)
	}

	return account, project, nil
}
