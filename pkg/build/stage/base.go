package stage

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flant/werf/pkg/container_runtime"

	"github.com/flant/logboek"

	"github.com/flant/werf/pkg/config"
	imagePkg "github.com/flant/werf/pkg/image"
	"github.com/flant/werf/pkg/slug"
	"github.com/flant/werf/pkg/util"
	"github.com/flant/werf/pkg/werf"
)

type StageName string

const (
	From                 StageName = "from"
	BeforeInstall        StageName = "beforeInstall"
	ImportsBeforeInstall StageName = "importsBeforeInstall"
	GitArchive           StageName = "gitArchive"
	Install              StageName = "install"
	ImportsAfterInstall  StageName = "importsAfterInstall"
	BeforeSetup          StageName = "beforeSetup"
	ImportsBeforeSetup   StageName = "importsBeforeSetup"
	Setup                StageName = "setup"
	ImportsAfterSetup    StageName = "importsAfterSetup"
	GitCache             StageName = "gitCache"
	GitLatestPatch       StageName = "gitLatestPatch"
	DockerInstructions   StageName = "dockerInstructions"

	Dockerfile StageName = "dockerfile"
)

var (
	AllStages = []StageName{
		From,
		BeforeInstall,
		ImportsBeforeInstall,
		GitArchive,
		Install,
		ImportsAfterInstall,
		BeforeSetup,
		ImportsBeforeSetup,
		Setup,
		ImportsAfterSetup,
		GitCache,
		GitLatestPatch,
		DockerInstructions,

		Dockerfile,
	}
)

type NewBaseStageOptions struct {
	ImageName        string
	ConfigMounts     []*config.Mount
	ImageTmpDir      string
	ContainerWerfDir string
	ProjectName      string
}

func newBaseStage(name StageName, options *NewBaseStageOptions) *BaseStage {
	s := &BaseStage{}
	s.name = name
	s.imageName = options.ImageName
	s.configMounts = options.ConfigMounts
	s.imageTmpDir = options.ImageTmpDir
	s.containerWerfDir = options.ContainerWerfDir
	s.projectName = options.ProjectName
	return s
}

type BaseStage struct {
	name             StageName
	imageName        string
	signature        string
	image            container_runtime.ImageInterface
	gitMappings      []*GitMapping
	imageTmpDir      string
	containerWerfDir string
	configMounts     []*config.Mount
	projectName      string

	PrevStage         Interface
	NextStage         Interface
	PrevBuiltStage    Interface
	PrevNonEmptyStage Interface
	NextNonEmptyStage Interface
	PrevImage         container_runtime.ImageInterface
	PrevBuiltImage    container_runtime.ImageInterface
}

func (s *BaseStage) LogDetailedName() string {
	imageName := s.imageName
	if imageName == "" {
		imageName = "~"
	}

	return fmt.Sprintf("%s/%s", imageName, s.Name())
}

func (s *BaseStage) Name() StageName {
	if s.name != "" {
		return s.name
	}

	panic("name must be defined!")
}

func (s *BaseStage) GetDependencies(_ Conveyor, _, _ container_runtime.ImageInterface) (string, error) {
	panic("method must be implemented!")
}

func (s *BaseStage) GetNextStageDependencies(_ Conveyor) (string, error) {
	return "", nil
}

func (s *BaseStage) getNextStageGitDependencies(_ Conveyor) (string, error) {
	var args []string
	for _, gitMapping := range s.gitMappings {
		if s.image.GetStagesStorageImageInfo() != nil {
			args = append(args, gitMapping.GetGitCommitFromImageLabels(s.image.GetStagesStorageImageInfo().Labels))
		} else {
			latestCommit, err := gitMapping.LatestCommit()
			if err != nil {
				return "", fmt.Errorf("unable to get latest commit of git mapping %s: %s", gitMapping.Name, err)
			}
			args = append(args, latestCommit)
		}
	}

	logboek.Debug.LogF("Stage %q next stage dependencies: %#v\n", s.Name(), args)
	sort.Strings(args)

	return util.Sha256Hash(args...), nil
}

func (s *BaseStage) IsEmpty(_ Conveyor, _ container_runtime.ImageInterface) (bool, error) {
	return false, nil
}

func (s *BaseStage) ShouldBeReset(builtImage container_runtime.ImageInterface) (bool, error) {
	for _, gitMapping := range s.gitMappings {
		commit := gitMapping.GetGitCommitFromImageLabels(builtImage.GetStagesStorageImageInfo().Labels)
		if commit == "" {
			return false, nil
		} else if exist, err := gitMapping.GitRepo().IsCommitExists(commit); err != nil {
			return false, err
		} else if !exist {
			return true, nil
		}
	}

	return false, nil
}

func (s *BaseStage) selectCacheImageByOldestCreationTimestamp(images []*imagePkg.Info) (*imagePkg.Info, error) {
	var oldestImage *imagePkg.Info
	for _, img := range images {
		if oldestImage == nil {
			oldestImage = img
		} else if img.GetCreatedAt().Before(oldestImage.GetCreatedAt()) {
			oldestImage = img
		}
	}
	return oldestImage, nil
}

func (s *BaseStage) selectCacheImagesAncestorsByGitMappings(images []*imagePkg.Info) ([]*imagePkg.Info, error) {
	suitableImages := []*imagePkg.Info{}
	currentCommits := make(map[string]string)

	for _, gitMapping := range s.gitMappings {
		currentCommit, err := gitMapping.LatestCommit()
		if err != nil {
			return nil, fmt.Errorf("error getting latest commit of git mapping %s: %s", gitMapping.Name, err)
		}
		currentCommits[gitMapping.Name] = currentCommit
	}

ScanImages:
	for _, img := range images {
		for _, gitMapping := range s.gitMappings {
			currentCommit := currentCommits[gitMapping.Name]

			commit := gitMapping.GetGitCommitFromImageLabels(img.Labels)
			if commit != "" {
				isOurAncestor, err := gitMapping.GitRepo().IsAncestor(commit, currentCommit)
				if err != nil {
					return nil, fmt.Errorf("error checking commits ancestry %s<-%s: %s", commit, currentCommit, err)
				}

				if !isOurAncestor {
					logboek.Debug.LogF("%s is not ancestor of %s for git repo %s: ignore image %s\n", commit, currentCommit, gitMapping.GitRepo().String(), img.Name)
					continue ScanImages
				}

				logboek.Debug.LogF(
					"%s is ancestor of %s for git repo %s: image %s is suitable for git archive stage\n",
					commit, currentCommit, gitMapping.GitRepo().String(), img.Name,
				)
			} else {
				logboek.Debug.LogF("WARNING: No git commit found in image %s, skipping\n", img.Name)
				continue ScanImages
			}
		}

		suitableImages = append(suitableImages, img)
	}

	return suitableImages, nil
}

func (s *BaseStage) SelectCacheImage(images []*imagePkg.Info) (*imagePkg.Info, error) {
	return s.selectCacheImageByOldestCreationTimestamp(images)
}

func (s *BaseStage) PrepareImage(_ Conveyor, prevBuiltImage, image container_runtime.ImageInterface) error {
	/*
	 * NOTE: BaseStage.PrepareImage does not called in From.PrepareImage.
	 * NOTE: Take into account when adding new base PrepareImage steps.
	 */

	serviceMounts := s.getServiceMounts(prevBuiltImage)
	s.addServiceMountsLabels(serviceMounts, image)
	if err := s.addServiceMountsVolumes(serviceMounts, image); err != nil {
		return fmt.Errorf("error adding mounts volumes: %s", err)
	}

	customMounts := s.getCustomMounts(prevBuiltImage)
	s.addCustomMountLabels(customMounts, image)
	if err := s.addCustomMountVolumes(customMounts, image); err != nil {
		return fmt.Errorf("error adding mounts volumes: %s", err)
	}

	return nil
}

func (s *BaseStage) AfterSignatureCalculated(_ Conveyor) error {
	return nil
}

func (s *BaseStage) PreRunHook(_ Conveyor) error {
	return nil
}

func (s *BaseStage) getServiceMounts(prevBuiltImage container_runtime.ImageInterface) map[string][]string {
	return mergeMounts(s.getServiceMountsFromLabels(prevBuiltImage), s.getServiceMountsFromConfig())
}

func (s *BaseStage) getServiceMountsFromLabels(prevBuiltImage container_runtime.ImageInterface) map[string][]string {
	mountpointsByType := map[string][]string{}

	var labels map[string]string
	if prevBuiltImage != nil {
		labels = prevBuiltImage.GetStagesStorageImageInfo().Labels
	}

	for _, labelMountType := range []struct{ Label, MountType string }{
		{imagePkg.WerfMountTmpDirLabel, "tmp_dir"},
		{imagePkg.WerfMountBuildDirLabel, "build_dir"},
	} {
		v, hasKey := labels[labelMountType.Label]
		if !hasKey {
			continue
		}

		mountpoints := util.RejectEmptyStrings(util.UniqStrings(strings.Split(v, ";")))
		mountpointsByType[labelMountType.MountType] = mountpoints
	}

	return mountpointsByType
}

func (s *BaseStage) getServiceMountsFromConfig() map[string][]string {
	mountpointsByType := map[string][]string{}

	for _, mountCfg := range s.configMounts {
		if !util.IsStringsContainValue([]string{"tmp_dir", "build_dir"}, mountCfg.Type) {
			continue
		}

		mountpoint := path.Clean(mountCfg.To)
		mountpointsByType[mountCfg.Type] = append(mountpointsByType[mountCfg.Type], mountpoint)
	}

	return mountpointsByType
}

func (s *BaseStage) addServiceMountsVolumes(mountpointsByType map[string][]string, image container_runtime.ImageInterface) error {
	for mountType, mountpoints := range mountpointsByType {
		for _, mountpoint := range mountpoints {
			absoluteMountpoint := path.Join("/", mountpoint)

			var absoluteFrom string
			switch mountType {
			case "tmp_dir":
				absoluteFrom = filepath.Join(s.imageTmpDir, "mount", slug.Slug(absoluteMountpoint))
			case "build_dir":
				absoluteFrom = filepath.Join(werf.GetSharedContextDir(), "mounts", "projects", s.projectName, slug.Slug(absoluteMountpoint))
			default:
				panic(fmt.Sprintf("unknown mount type %s", mountType))
			}

			err := os.MkdirAll(absoluteFrom, os.ModePerm)
			if err != nil {
				return fmt.Errorf("error creating tmp path %s for mount: %s", absoluteFrom, err)
			}

			image.Container().RunOptions().AddVolume(fmt.Sprintf("%s:%s", absoluteFrom, absoluteMountpoint))
		}
	}

	return nil
}

func (s *BaseStage) addServiceMountsLabels(mountpointsByType map[string][]string, image container_runtime.ImageInterface) {
	for mountType, mountpoints := range mountpointsByType {
		var labelName string
		switch mountType {
		case "tmp_dir":
			labelName = imagePkg.WerfMountTmpDirLabel
		case "build_dir":
			labelName = imagePkg.WerfMountBuildDirLabel
		default:
			panic(fmt.Sprintf("unknown mount type %s", mountType))
		}

		labelValue := strings.Join(mountpoints, ";")

		image.Container().ServiceCommitChangeOptions().AddLabel(map[string]string{labelName: labelValue})
	}
}

func (s *BaseStage) getCustomMounts(prevBuiltImage container_runtime.ImageInterface) map[string][]string {
	return mergeMounts(s.getCustomMountsFromLabels(prevBuiltImage), s.getCustomMountsFromConfig())
}

func (s *BaseStage) getCustomMountsFromLabels(prevBuiltImage container_runtime.ImageInterface) map[string][]string {
	mountpointsByFrom := map[string][]string{}

	var labels map[string]string
	if prevBuiltImage != nil {
		labels = prevBuiltImage.GetStagesStorageImageInfo().Labels
	}
	for k, v := range labels {
		if !strings.HasPrefix(k, imagePkg.WerfMountCustomDirLabelPrefix) {
			continue
		}

		parts := strings.SplitN(k, imagePkg.WerfMountCustomDirLabelPrefix, 2)
		from := strings.Replace(parts[1], "--", "/", -1)

		mountpoints := util.RejectEmptyStrings(util.UniqStrings(strings.Split(v, ";")))
		mountpointsByFrom[from] = mountpoints
	}

	return mountpointsByFrom
}

func (s *BaseStage) getCustomMountsFromConfig() map[string][]string {
	mountpointsByFrom := map[string][]string{}
	for _, mountCfg := range s.configMounts {
		if mountCfg.Type != "custom_dir" {
			continue
		}

		from := filepath.Clean(mountCfg.From)
		mountpoint := path.Clean(mountCfg.To)

		mountpointsByFrom[from] = util.UniqAppendString(mountpointsByFrom[from], mountpoint)
	}

	return mountpointsByFrom
}

func (s *BaseStage) addCustomMountVolumes(mountpointsByFrom map[string][]string, image container_runtime.ImageInterface) error {
	for from, mountpoints := range mountpointsByFrom {
		absoluteFrom := util.ExpandPath(from)

		exist, err := util.FileExists(absoluteFrom)
		if err != nil {
			return err
		}

		if !exist {
			err := os.MkdirAll(absoluteFrom, os.ModePerm)
			if err != nil {
				return fmt.Errorf("error creating %s: %s", absoluteFrom, err)
			}
		}

		for _, mountpoint := range mountpoints {
			absoluteMountpoint := path.Join("/", mountpoint)
			image.Container().RunOptions().AddVolume(fmt.Sprintf("%s:%s", absoluteFrom, absoluteMountpoint))
		}
	}

	return nil
}

func (s *BaseStage) addCustomMountLabels(mountpointsByFrom map[string][]string, image container_runtime.ImageInterface) {
	for from, mountpoints := range mountpointsByFrom {
		labelName := fmt.Sprintf("%s%s", imagePkg.WerfMountCustomDirLabelPrefix, strings.Replace(from, "/", "--", -1))
		labelValue := strings.Join(mountpoints, ";")
		image.Container().ServiceCommitChangeOptions().AddLabel(map[string]string{labelName: labelValue})
	}
}

func (s *BaseStage) SetSignature(signature string) {
	s.signature = signature
}

func (s *BaseStage) GetSignature() string {
	return s.signature
}

func (s *BaseStage) SetImage(image container_runtime.ImageInterface) {
	s.image = image
}

func (s *BaseStage) GetImage() container_runtime.ImageInterface {
	return s.image
}

func (s *BaseStage) SetGitMappings(gitMappings []*GitMapping) {
	s.gitMappings = gitMappings
}

func (s *BaseStage) GetGitMappings() []*GitMapping {
	return s.gitMappings
}

func mergeMounts(a, b map[string][]string) map[string][]string {
	res := map[string][]string{}

	for k, mountpoints := range a {
		res[k] = mountpoints
	}
	for k, mountpoints := range b {
		res[k] = util.UniqStrings(append(res[k], mountpoints...))
	}

	return res
}

func (s *BaseStage) GetPrevStage() Interface {
	return s.PrevStage
}

func (s *BaseStage) GetPrevBuiltStage() Interface {
	return s.PrevBuiltStage
}

func (s *BaseStage) GetPrevNonEmptyStage() Interface {
	return s.PrevNonEmptyStage
}

func (s *BaseStage) GetPrevImage() container_runtime.ImageInterface {
	return s.PrevImage
}

func (s *BaseStage) GetPrevBuiltImage() container_runtime.ImageInterface {
	return s.PrevBuiltImage
}

func (s *BaseStage) GetNextStage() Interface {
	return s.NextStage
}

func (s *BaseStage) GetNextNonEmptyStage() Interface {
	return s.NextNonEmptyStage
}

func (s *BaseStage) SetPrevStage(stage Interface) {
	s.PrevStage = stage
}

func (s *BaseStage) SetPrevBuiltStage(stage Interface) {
	s.PrevBuiltStage = stage
}

func (s *BaseStage) SetPrevNonEmptyStage(stage Interface) {
	s.PrevNonEmptyStage = stage
}

func (s *BaseStage) SetPrevImage(image container_runtime.ImageInterface) {
	s.PrevImage = image
}

func (s *BaseStage) SetPrevBuiltImage(image container_runtime.ImageInterface) {
	s.PrevBuiltImage = image
}

func (s *BaseStage) SetNextStage(stage Interface) {
	s.NextStage = stage
}

func (s *BaseStage) SetNextNonEmptyStage(stage Interface) {
	s.NextNonEmptyStage = stage
}
