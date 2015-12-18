package graph

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/registry"
)

// Pusher is an interface that abstracts pushing for different API versions.
type ManifestPusher interface {
	// Push tries to push the image configured at the creation of Pusher.
	// Push returns an error if any, as well as a boolean that determines whether to retry Push on the next configured endpoint.
	//
	Push(newTag string) (fallback bool, err error)
}

// NewPusher creates a new Pusher interface that will push to a v2
// registry. The endpoint argument contains a Version field that determines
// whether a v2 pusher will be created. The other parameters are passed
// through to the underlying pusher implementation for use during the actual
// push operation.
func (s *TagStore) NewManifestPusher(endpoint registry.APIEndpoint, localRepo Repository, repoInfo *registry.RepositoryInfo, imagePushConfig *ImagePushConfig, sf *streamformatter.StreamFormatter) (Pusher, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2ManifestPusher{
			TagStore:     s,
			endpoint:     endpoint,
			localRepo:    localRepo,
			repoInfo:     repoInfo,
			config:       imagePushConfig,
			sf:           sf,
			layersPushed: make(map[digest.Digest]bool),
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

// Push initiates a push operation on the repository named localName.
func (s *TagStore) PushManifest(localName string, newTag string, imagePushConfig *ImagePushConfig) error {
	fmt.Println("In graph.PushManifest")
	fmt.Printf("localName: %s\n", localName)
	fmt.Printf("NewTag: %s\n", newTag)
	fmt.Printf("imagePushConfig: %s\n", imagePushConfig)
	// FIXME: Allow to interrupt current push when new push of same image is done.

	var sf = streamformatter.NewJSONStreamFormatter()

	// Resolve the Repository name from fqn to RepositoryInfo
	repoInfo, err := s.registryService.ResolveRepository(localName)
	if err != nil {
		return err
	}

	endpoints, err := s.registryService.LookupPushEndpoints(repoInfo.CanonicalName)
	if err != nil {
		return err
	}

	reposLen := 1
	if imagePushConfig.Tag == "" {
		reposLen = len(s.Repositories[repoInfo.LocalName])
	}

	imagePushConfig.OutStream.Write(sf.FormatStatus("", "The push refers to a repository [%s] (len: %d)", repoInfo.CanonicalName, reposLen))

	// If it fails, try to get the repository
	localRepo, exists := s.Repositories[repoInfo.LocalName]
	if !exists {
		return fmt.Errorf("Repository does not exist: %s", repoInfo.LocalName)
	}

	var lastErr error
	for _, endpoint := range endpoints {
		logrus.Debugf("Trying to push %s to %s %s", repoInfo.CanonicalName, endpoint.URL, endpoint.Version)

		pusher, err := s.NewManifestPusher(endpoint, localRepo, repoInfo, imagePushConfig, sf)
		if err != nil {
			lastErr = err
			continue
		}
		if fallback, err := pusher.Push(); err != nil {
			if fallback {
				lastErr = err
				continue
			}
			logrus.Debugf("Not continuing with error: %v", err)
			return err

		}

		s.eventsService.Log("push", repoInfo.LocalName, "")
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", repoInfo.CanonicalName)
	}
	return lastErr
}
