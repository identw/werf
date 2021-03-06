package stage

import (
	"fmt"

	"github.com/flant/werf/pkg/image"
	"github.com/flant/werf/pkg/util"
)

func NewGitLatestPatchStage(gitPatchStageOptions *NewGitPatchStageOptions, baseStageOptions *NewBaseStageOptions) *GitLatestPatchStage {
	s := &GitLatestPatchStage{}
	s.GitPatchStage = newGitPatchStage(GitLatestPatch, gitPatchStageOptions, baseStageOptions)
	return s
}

type GitLatestPatchStage struct {
	*GitPatchStage
}

func (s *GitLatestPatchStage) IsEmpty(c Conveyor, prevBuiltImage image.ImageInterface) (bool, error) {
	if empty, err := s.GitPatchStage.IsEmpty(c, prevBuiltImage); err != nil {
		return false, err
	} else if empty {
		return true, nil
	}

	isEmpty := true
	for _, gitMapping := range s.gitMappings {
		commit := gitMapping.GetGitCommitFromImageLabels(prevBuiltImage.Labels())
		if exist, err := gitMapping.GitRepo().IsCommitExists(commit); err != nil {
			return false, err
		} else if !exist {
			return true, nil
		}

		if empty, err := gitMapping.IsPatchEmpty(prevBuiltImage); err != nil {
			return false, err
		} else if !empty {
			isEmpty = false
			break
		}
	}

	return isEmpty, nil
}

func (s *GitLatestPatchStage) GetDependencies(_ Conveyor, _, prevBuiltImage image.ImageInterface) (string, error) {
	var args []string

	for _, gitMapping := range s.gitMappings {
		patchContent, err := gitMapping.GetPatchContent(prevBuiltImage)
		if err != nil {
			return "", fmt.Errorf("error getting patch between previous built image %s and current commit for git mapping %s: %s", prevBuiltImage.Name(), gitMapping.Name, err)
		}

		args = append(args, patchContent)
	}

	return util.Sha256Hash(args...), nil
}
