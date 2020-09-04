package cleaning

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rodaine/table"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/werf/lockgate"
	"github.com/werf/logboek"

	"github.com/werf/werf/pkg/config"
	"github.com/werf/werf/pkg/image"
	"github.com/werf/werf/pkg/logging"
	"github.com/werf/werf/pkg/slug"
	"github.com/werf/werf/pkg/stages_manager"
	"github.com/werf/werf/pkg/storage"
	"github.com/werf/werf/pkg/tag_strategy"
	"github.com/werf/werf/pkg/werf"
)

type ImagesCleanupOptions struct {
	ImageNameList                 []string
	LocalGit                      GitRepo
	KubernetesContextsClients     map[string]kubernetes.Interface
	KubernetesNamespaces          map[string]string
	WithoutKube                   bool
	Policies                      ImagesCleanupPolicies
	GitHistoryBasedCleanup        bool
	GitHistoryBasedCleanupV12     bool
	GitHistoryBasedCleanupOptions config.MetaCleanup
	DryRun                        bool
}

func ImagesCleanup(projectName string, imagesRepo storage.ImagesRepo, stagesManager *stages_manager.StagesManager, storageLockManager storage.LockManager, options ImagesCleanupOptions) error {
	m := newImagesCleanupManager(projectName, imagesRepo, stagesManager, options)

	if lock, err := storageLockManager.LockStagesAndImages(projectName, storage.LockStagesAndImagesOptions{GetOrCreateImagesOnly: false}); err != nil {
		return fmt.Errorf("unable to lock stages and images: %s", err)
	} else {
		defer storageLockManager.Unlock(lock)
	}

	return logboek.Default.LogProcess(
		"Running images cleanup",
		logboek.LevelLogProcessOptions{Style: logboek.HighlightStyle()},
		m.run,
	)
}

func newImagesCleanupManager(projectName string, imagesRepo storage.ImagesRepo, stagesManager *stages_manager.StagesManager, options ImagesCleanupOptions) *imagesCleanupManager {
	return &imagesCleanupManager{
		ProjectName:                   projectName,
		ImagesRepo:                    imagesRepo,
		StagesManager:                 stagesManager,
		ImageNameList:                 options.ImageNameList,
		DryRun:                        options.DryRun,
		LocalGit:                      options.LocalGit,
		KubernetesContextsClients:     options.KubernetesContextsClients,
		KubernetesNamespaces:          options.KubernetesNamespaces,
		WithoutKube:                   options.WithoutKube,
		Policies:                      options.Policies,
		GitHistoryBasedCleanup:        options.GitHistoryBasedCleanup,
		GitHistoryBasedCleanupV12:     options.GitHistoryBasedCleanupV12,
		GitHistoryBasedCleanupOptions: options.GitHistoryBasedCleanupOptions,
	}
}

type imagesCleanupManager struct {
	imageRepoImageList           *map[string][]*image.Info
	imageCommitHashImageMetadata *map[string]map[plumbing.Hash]*storage.ImageMetadata

	ProjectName                   string
	ImagesRepo                    storage.ImagesRepo
	StagesManager                 *stages_manager.StagesManager
	ImageNameList                 []string
	LocalGit                      GitRepo
	KubernetesContextsClients     map[string]kubernetes.Interface
	KubernetesNamespaces          map[string]string
	WithoutKube                   bool
	Policies                      ImagesCleanupPolicies
	GitHistoryBasedCleanup        bool
	GitHistoryBasedCleanupV12     bool
	GitHistoryBasedCleanupOptions config.MetaCleanup
	DryRun                        bool
}

type GitRepo interface {
	PlainOpen() (*git.Repository, error)
	IsCommitExists(commit string) (bool, error)
	TagsList() ([]string, error)
	RemoteBranchesList() ([]string, error)
}

type ImagesCleanupPolicies struct {
	GitTagStrategyHasLimit bool // No limit by default!
	GitTagStrategyLimit    int64

	GitTagStrategyHasExpiryPeriod bool // No expiration by default!
	GitTagStrategyExpiryPeriod    time.Duration

	GitCommitStrategyHasLimit bool // No limit by default!
	GitCommitStrategyLimit    int64

	GitCommitStrategyHasExpiryPeriod bool // No expiration by default!
	GitCommitStrategyExpiryPeriod    time.Duration

	StagesSignatureStrategyHasLimit bool // No limit by default!
	StagesSignatureStrategyLimit    int64

	StagesSignatureStrategyHasExpiryPeriod bool // No expiration by default!
	StagesSignatureStrategyExpiryPeriod    time.Duration
}

func (m *imagesCleanupManager) initRepoImagesData() error {
	if err := logboek.Info.LogProcess("Fetching repo images", logboek.LevelLogProcessOptions{}, m.initRepoImages); err != nil {
		return err
	}

	if m.GitHistoryBasedCleanup || m.GitHistoryBasedCleanupV12 {
		if err := logboek.Info.LogProcess("Fetching images metadata", logboek.LevelLogProcessOptions{}, m.initImageCommitHashImageMetadata); err != nil {
			return err
		}
	}

	return nil
}

func (m *imagesCleanupManager) initRepoImages() error {
	repoImages, err := selectRepoImagesFromImagesRepo(m.ImagesRepo, m.ImageNameList)
	if err != nil {
		return err
	}

	m.setImageRepoImageList(repoImages)

	return nil
}

func (m *imagesCleanupManager) initImageCommitHashImageMetadata() error {
	imageCommitImageMetadata := map[string]map[plumbing.Hash]*storage.ImageMetadata{}
	for _, imageName := range m.ImageNameList {
		commits, err := m.StagesManager.StagesStorage.GetImageCommits(m.ProjectName, imageName)
		if err != nil {
			return fmt.Errorf("get image %s commits failed: %s", imageName, err)
		}

		commitImageMetadata := map[plumbing.Hash]*storage.ImageMetadata{}
		for _, commit := range commits {
			imageMetadata, err := m.StagesManager.StagesStorage.GetImageMetadataByCommit(m.ProjectName, imageName, commit)
			if err != nil {
				return fmt.Errorf("get image %s metadata by commit %s failed", imageName, commit)
			}

			if imageMetadata != nil {
				commitImageMetadata[plumbing.NewHash(commit)] = imageMetadata
			}
		}

		imageCommitImageMetadata[imageName] = commitImageMetadata
	}

	m.setImageCommitImageMetadata(imageCommitImageMetadata)

	return nil
}

func (m *imagesCleanupManager) getImageRepoImageList() map[string][]*image.Info {
	return *m.imageRepoImageList
}

func (m *imagesCleanupManager) setImageRepoImageList(repoImages map[string][]*image.Info) {
	m.imageRepoImageList = &repoImages
}

func (m *imagesCleanupManager) getImageCommitHashImageMetadata() map[string]map[plumbing.Hash]*storage.ImageMetadata {
	return *m.imageCommitHashImageMetadata
}

func (m *imagesCleanupManager) setImageCommitImageMetadata(imageCommitImageMetadata map[string]map[plumbing.Hash]*storage.ImageMetadata) {
	m.imageCommitHashImageMetadata = &imageCommitImageMetadata
}

func (m *imagesCleanupManager) run() error {
	imagesCleanupLockName := fmt.Sprintf("images-cleanup.%s", m.ImagesRepo.String())
	return werf.WithHostLock(imagesCleanupLockName, lockgate.AcquireOptions{Timeout: time.Second * 600}, func() error {
		if err := logboek.LogProcess("Fetching repo images data", logboek.LogProcessOptions{}, m.initRepoImagesData); err != nil {
			return err
		}

		repoImagesToCleanup := m.getImageRepoImageList()
		exceptedRepoImages := map[string][]*image.Info{}
		resultRepoImages := map[string][]*image.Info{}

		if m.LocalGit == nil {
			logboek.Default.LogLnDetails("Images cleanup skipped due to local git repository was not detected")
			return nil
		}

		var err error

		if !m.WithoutKube {
			if err := logboek.LogProcess("Skipping repo images that are being used in Kubernetes", logboek.LogProcessOptions{}, func() error {
				repoImagesToCleanup, exceptedRepoImages, err = exceptRepoImagesByWhitelist(repoImagesToCleanup, m.KubernetesContextsClients, m.KubernetesNamespaces)

				return err
			}); err != nil {
				return err
			}
		}

		if m.GitHistoryBasedCleanup || m.GitHistoryBasedCleanupV12 {
			resultRepoImages, err = m.repoImagesGitHistoryBasedCleanup(repoImagesToCleanup)
			if err != nil {
				return err
			}
		} else {
			resultRepoImages, err = m.repoImagesCleanup(repoImagesToCleanup)
			if err != nil {
				return err
			}
		}

		for imageName, repoImageList := range exceptedRepoImages {
			_, ok := resultRepoImages[imageName]
			if !ok {
				resultRepoImages[imageName] = repoImageList
			} else {
				resultRepoImages[imageName] = append(resultRepoImages[imageName], repoImageList...)
			}
		}

		m.setImageRepoImageList(resultRepoImages)

		return nil
	})
}

func exceptRepoImagesByWhitelist(repoImages map[string][]*image.Info, kubernetesContextsClients map[string]kubernetes.Interface, kubernetesNamespaces map[string]string) (map[string][]*image.Info, map[string][]*image.Info, error) {
	deployedDockerImagesNames, err := getDeployedDockerImagesNames(kubernetesContextsClients, kubernetesNamespaces)
	if err != nil {
		return nil, nil, err
	}

	exceptedRepoImages := map[string][]*image.Info{}
	for imageName, repoImageList := range repoImages {
		var newRepoImages []*image.Info

		_ = logboek.Default.LogBlock(logging.ImageLogProcessName(imageName, false), logboek.LevelLogBlockOptions{}, func() error {

		Loop:
			for _, repoImage := range repoImageList {
				dockerImageName := fmt.Sprintf("%s:%s", repoImage.Repository, repoImage.Tag)
				for _, deployedDockerImageName := range deployedDockerImagesNames {
					if deployedDockerImageName == dockerImageName {
						exceptedImageList, ok := exceptedRepoImages[imageName]
						if !ok {
							exceptedImageList = []*image.Info{}
						}

						exceptedImageList = append(exceptedImageList, repoImage)
						exceptedRepoImages[imageName] = exceptedImageList

						logboek.Default.LogFDetails("  tag: %s\n", repoImage.Tag)
						logboek.LogOptionalLn()
						continue Loop
					}
				}

				newRepoImages = append(newRepoImages, repoImage)
			}

			repoImages[imageName] = newRepoImages

			return nil
		})
	}

	return repoImages, exceptedRepoImages, nil
}

func getDeployedDockerImagesNames(kubernetesContextsClients map[string]kubernetes.Interface, kubernetesNamespaces map[string]string) ([]string, error) {
	var deployedDockerImagesNames []string
	for contextName, kubernetesClient := range kubernetesContextsClients {
		if err := logboek.LogProcessInline(fmt.Sprintf("Getting deployed docker images (context %s)", contextName), logboek.LogProcessInlineOptions{}, func() error {
			kubernetesClientDeployedDockerImagesNames, err := deployedDockerImages(kubernetesClient, kubernetesNamespaces[contextName])
			if err != nil {
				return fmt.Errorf("cannot get deployed imagesRepoImageList: %s", err)
			}

			deployedDockerImagesNames = append(deployedDockerImagesNames, kubernetesClientDeployedDockerImagesNames...)

			return nil
		}); err != nil {
			return nil, err
		}
	}

	return deployedDockerImagesNames, nil
}

func (m *imagesCleanupManager) repoImagesCleanup(repoImagesToCleanup map[string][]*image.Info) (map[string][]*image.Info, error) {
	resultRepoImages := map[string][]*image.Info{}

	for imageName, repoImageListToCleanup := range repoImagesToCleanup {
		logProcessMessage := fmt.Sprintf("Processing %s", logging.ImageLogProcessName(imageName, false))
		if err := logboek.Default.LogProcess(
			logProcessMessage,
			logboek.LevelLogProcessOptions{Style: logboek.HighlightStyle()},
			func() error {
				repoImageListToCleanup, err := m.repoImagesCleanupByNonexistentGitPrimitive(repoImageListToCleanup)
				if err != nil {
					return err
				}

				resultRepoImageList, err := m.repoImagesCleanupByPolicies(repoImageListToCleanup)
				if err != nil {
					return err
				}
				resultRepoImages[imageName] = resultRepoImageList

				return nil
			},
		); err != nil {
			return nil, err
		}
	}

	return resultRepoImages, nil
}

func (m *imagesCleanupManager) repoImagesGitHistoryBasedCleanup(repoImagesToCleanup map[string][]*image.Info) (map[string][]*image.Info, error) {
	resultRepoImages := map[string][]*image.Info{}

	gitRepository, err := m.LocalGit.PlainOpen()
	if err != nil {
		return nil, fmt.Errorf("git plain open failed: %s", err)
	}

	var referencesToScan []*referenceToScan
	if err := logboek.Default.LogProcess("Preparing references to scan", logboek.LevelLogProcessOptions{}, func() error {
		referencesToScan, err = getReferencesToScan(gitRepository, m.GitHistoryBasedCleanupOptions.KeepPolicies)
		return err
	}); err != nil {
		return nil, err
	}

	var imageContentSignatureRepoImageListToCleanup map[string]map[string][]*image.Info
	var imageContentSignatureExistingCommitHashes map[string]map[string][]plumbing.Hash
	if err := logboek.Info.LogProcess("Grouping repo images tags by content signature", logboek.LevelLogProcessOptions{}, func() error {
		imageContentSignatureRepoImageListToCleanup, err = m.getImageContentSignatureRepoImageListToCleanup(repoImagesToCleanup)
		if err != nil {
			return err
		}

		imageContentSignatureExistingCommitHashes, err = m.getImageContentSignatureExistingCommitHashes()
		if err != nil {
			return err
		}

		if logboek.Info.IsAccepted() {
			for imageName, contentSignatureRepoImageListToCleanup := range imageContentSignatureRepoImageListToCleanup {
				if len(contentSignatureRepoImageListToCleanup) == 0 {
					continue
				}

				logboek.Info.LogProcessStart(logging.ImageLogProcessName(imageName, false), logboek.LevelLogProcessStartOptions{})

				var rows [][]interface{}
				for contentSignature, repoImageListToCleanup := range contentSignatureRepoImageListToCleanup {
					commitHashes := imageContentSignatureExistingCommitHashes[imageName][contentSignature]
					if len(commitHashes) == 0 || len(repoImageListToCleanup) == 0 {
						continue
					}

					var maxInd int
					for _, length := range []int{len(commitHashes), len(repoImageListToCleanup)} {
						if length > maxInd {
							maxInd = length
						}
					}

					shortify := func(column string) string {
						if len(column) > 15 {
							return fmt.Sprintf("%s..%s", column[:10], column[len(column)-3:])
						} else {
							return column
						}
					}

					for ind := 0; ind < maxInd; ind++ {
						var columns []interface{}
						if ind == 0 {
							columns = append(columns, shortify(contentSignature))
						} else {
							columns = append(columns, "")
						}

						if len(commitHashes) > ind {
							columns = append(columns, shortify(commitHashes[ind].String()))
						} else {
							columns = append(columns, "")
						}

						if len(repoImageListToCleanup) > ind {
							column := repoImageListToCleanup[ind].Tag
							if logboek.ContentWidth() < 100 {
								column = shortify(column)
							}
							columns = append(columns, column)
						} else {
							columns = append(columns, "")
						}

						rows = append(rows, columns)
					}
				}

				if len(rows) != 0 {
					tbl := table.New("Content Signature", "Existing Commits", "Tags")
					tbl.WithWriter(logboek.GetOutStream())
					tbl.WithHeaderFormatter(color.New(color.Underline).SprintfFunc())
					for _, row := range rows {
						tbl.AddRow(row...)
					}
					tbl.Print()

					logboek.LogOptionalLn()
				}

				for contentSignature, repoImageListToCleanup := range contentSignatureRepoImageListToCleanup {
					commitHashes := imageContentSignatureExistingCommitHashes[imageName][contentSignature]
					if len(commitHashes) == 0 {
						logBlockMessage := fmt.Sprintf("Content signature %s is associated with non-existing commits. The following tags will be deleted", contentSignature)
						_ = logboek.Info.LogBlock(logBlockMessage, logboek.LevelLogBlockOptions{}, func() error {
							for _, repoImage := range repoImageListToCleanup {
								logboek.Info.LogFDetails("  tag: %s\n", repoImage.Tag)
								logboek.LogOptionalLn()
							}

							return nil
						})
					}
				}

				logboek.Info.LogProcessEnd(logboek.LevelLogProcessEndOptions{})
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err = logboek.Default.LogProcess("Processing images tags without related image metadata", logboek.LevelLogProcessOptions{}, func() error {
		imageRepoImageListWithoutRelatedContentSignature, err := m.getImageRepoImageListWithoutRelatedImageMetadata(repoImagesToCleanup, imageContentSignatureRepoImageListToCleanup)
		if err != nil {
			return err
		}

		for imageName, repoImages := range imageRepoImageListWithoutRelatedContentSignature {
			logboek.Default.LogProcessStart(logging.ImageLogProcessName(imageName, false), logboek.LevelLogProcessStartOptions{})

			if !m.GitHistoryBasedCleanupV12 {
				if len(repoImages) != 0 {
					logboek.Warn.LogF("Detected tags without related image metadata.\nThese tags will be saved during cleanup.\n")
					logboek.Warn.LogF("Since v1.2 git history based cleanup will delete such tags by default.\nYou can force this behaviour in current werf version with --git-history-based-cleanup-v1.2 option.\n")
				}

				for _, repoImage := range repoImages {
					logboek.Default.LogFDetails("  tag: %s\n", repoImage.Tag)
					logboek.LogOptionalLn()
				}

				resultRepoImages[imageName] = append(resultRepoImages[imageName], repoImages...)
				repoImagesToCleanup[imageName] = exceptRepoImageList(repoImagesToCleanup[imageName], repoImages...)
			}

			logboek.Default.LogProcessEnd(logboek.LevelLogProcessEndOptions{})
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err := logboek.Default.LogProcess("Git history based cleanup", logboek.LevelLogProcessOptions{
		Style: logboek.HighlightStyle(),
	}, func() error {
		for imageName, repoImageListToCleanup := range repoImagesToCleanup {
			var repoImageListToSave []*image.Info
			if err := logboek.LogProcess(logging.ImageLogProcessName(imageName, false), logboek.LogProcessOptions{}, func() error {
				if err := logboek.Default.LogProcess("Scanning git references history", logboek.LevelLogProcessOptions{}, func() error {
					contentSignatureCommitHashes := map[string][]plumbing.Hash{}
					contentSignatureRepoImageListToCleanup := imageContentSignatureRepoImageListToCleanup[imageName]
					for contentSignature, _ := range contentSignatureRepoImageListToCleanup {
						existingCommitHashes := imageContentSignatureExistingCommitHashes[imageName][contentSignature]
						if len(existingCommitHashes) == 0 {
							continue
						}

						contentSignatureCommitHashes[contentSignature] = existingCommitHashes
					}

					var repoImageListToKeep []*image.Info
					if len(contentSignatureCommitHashes) != 0 {
						reachedContentSignatureList, err := scanReferencesHistory(gitRepository, referencesToScan, contentSignatureCommitHashes)
						if err != nil {
							return err
						}

						for _, contentSignature := range reachedContentSignatureList {
							contentSignatureRepoImageListToCleanup, ok := imageContentSignatureRepoImageListToCleanup[imageName][contentSignature]
							if !ok {
								panic("runtime error")
							}

							repoImageListToKeep = append(repoImageListToKeep, contentSignatureRepoImageListToCleanup...)
						}

						repoImageListToSave = append(repoImageListToSave, repoImageListToKeep...)
						resultRepoImages[imageName] = append(resultRepoImages[imageName], repoImageListToKeep...)
						repoImageListToCleanup = exceptRepoImageList(repoImageListToCleanup, repoImageListToKeep...)
					} else {
						logboek.LogLn("Scanning stopped due to nothing to seek")
					}

					return nil
				}); err != nil {
					return err
				}

				if len(repoImageListToSave) != 0 {
					_ = logboek.Default.LogBlock("Saved tags", logboek.LevelLogBlockOptions{}, func() error {
						for _, repoImage := range repoImageListToSave {
							logboek.Default.LogFDetails("  tag: %s\n", repoImage.Tag)
							logboek.LogOptionalLn()
						}

						return nil
					})
				}

				if err := logboek.Default.LogProcess("Deleting tags", logboek.LevelLogProcessOptions{}, func() error {
					return deleteRepoImageInImagesRepo(m.ImagesRepo, m.DryRun, repoImageListToCleanup...)
				}); err != nil {
					return err
				}

				return nil
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err := logboek.Default.LogProcess("Deleting unused images metadata", logboek.LevelLogProcessOptions{}, func() error {
		imageUnusedCommitHashes, err := m.getImageUnusedCommitHashes(resultRepoImages)
		if err != nil {
			return err
		}

		for imageName, commitHashes := range imageUnusedCommitHashes {
			logboek.Default.LogProcessStart(logging.ImageLogProcessName(imageName, false), logboek.LevelLogProcessStartOptions{})

			for _, commitHash := range commitHashes {
				if m.DryRun {
					logboek.Default.LogLn(commitHash)
				} else {
					if err := m.StagesManager.StagesStorage.RmImageCommit(m.ProjectName, imageName, commitHash.String()); err != nil {
						logboek.Warn.LogF(
							"WARNING: Metadata image deletion (image %s, commit: %s) failed: %s\n",
							logging.ImageLogName(imageName, false), commitHash.String(), err,
						)
						logboek.LogOptionalLn()
					}
				}
			}

			logboek.Default.LogProcessEnd(logboek.LevelLogProcessEndOptions{})
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return resultRepoImages, nil
}

// getImageContentSignatureRepoImageListToCleanup groups images, content signatures and repo images tags to clean up.
// The map has all content signatures and each value is related to repo images tags to clean up and can be empty.
// Repo images tags to clean up are all tags for particular werf.yaml image except for ones which are using in Kubernetes.
func (m *imagesCleanupManager) getImageContentSignatureRepoImageListToCleanup(repoImagesToCleanup map[string][]*image.Info) (map[string]map[string][]*image.Info, error) {
	imageContentSignatureRepoImageListToCleanup := map[string]map[string][]*image.Info{}

	imageCommitHashImageMetadata := m.getImageCommitHashImageMetadata()
	for imageName, repoImageListToCleanup := range repoImagesToCleanup {
		imageContentSignatureRepoImageListToCleanup[imageName] = map[string][]*image.Info{}

		for _, imageMetadata := range imageCommitHashImageMetadata[imageName] {
			_, ok := imageContentSignatureRepoImageListToCleanup[imageName][imageMetadata.ContentSignature]
			if ok {
				continue
			}

			var repoImageListToCleanupBySignature []*image.Info
			for _, repoImage := range repoImageListToCleanup {
				if repoImage.Labels[image.WerfContentSignatureLabel] == imageMetadata.ContentSignature {
					repoImageListToCleanupBySignature = append(repoImageListToCleanupBySignature, repoImage)
				}
			}

			imageContentSignatureRepoImageListToCleanup[imageName][imageMetadata.ContentSignature] = repoImageListToCleanupBySignature
		}
	}

	return imageContentSignatureRepoImageListToCleanup, nil
}

func (m *imagesCleanupManager) repoImagesCleanupByNonexistentGitPrimitive(repoImages []*image.Info) ([]*image.Info, error) {
	var nonexistentGitTagRepoImages, nonexistentGitCommitRepoImages, nonexistentGitBranchRepoImages []*image.Info

	var gitTags []string
	var gitBranches []string

	if m.LocalGit != nil {
		var err error
		gitTags, err = m.LocalGit.TagsList()
		if err != nil {
			return nil, fmt.Errorf("cannot get local git tags list: %s", err)
		}

		gitBranches, err = m.LocalGit.RemoteBranchesList()
		if err != nil {
			return nil, fmt.Errorf("cannot get local git branches list: %s", err)
		}
	}

Loop:
	for _, repoImage := range repoImages {
		strategy, ok := repoImage.Labels[image.WerfTagStrategyLabel]
		if !ok {
			continue
		}

		repoImageMetaTag, ok := repoImage.Labels[image.WerfImageTagLabel]
		if !ok {
			repoImageMetaTag = repoImage.Tag
		}

		switch strategy {
		case string(tag_strategy.GitTag):
			if repoImageMetaTagMatch(repoImageMetaTag, gitTags...) {
				continue Loop
			} else {
				nonexistentGitTagRepoImages = append(nonexistentGitTagRepoImages, repoImage)
			}
		case string(tag_strategy.GitBranch):
			if repoImageMetaTagMatch(repoImageMetaTag, gitBranches...) {
				continue Loop
			} else {
				nonexistentGitBranchRepoImages = append(nonexistentGitBranchRepoImages, repoImage)
			}
		case string(tag_strategy.GitCommit):
			exist := false

			if m.LocalGit != nil {
				var err error

				exist, err = m.LocalGit.IsCommitExists(repoImageMetaTag)
				if err != nil {
					if strings.HasPrefix(err.Error(), "bad commit hash") {
						exist = false
					} else {
						return nil, err
					}
				}
			}

			if !exist {
				nonexistentGitCommitRepoImages = append(nonexistentGitCommitRepoImages, repoImage)
			}
		}
	}

	if len(nonexistentGitTagRepoImages) != 0 {
		if err := logboek.Default.LogBlock(
			"Removed tags by nonexistent git-tag policy",
			logboek.LevelLogBlockOptions{},
			func() error {
				return deleteRepoImageInImagesRepo(m.ImagesRepo, m.DryRun, nonexistentGitTagRepoImages...)
			},
		); err != nil {
			return nil, err
		}

		repoImages = exceptRepoImageList(repoImages, nonexistentGitTagRepoImages...)
	}

	if len(nonexistentGitBranchRepoImages) != 0 {
		if err := logboek.Default.LogBlock(
			"Removed tags by nonexistent git-branch policy",
			logboek.LevelLogBlockOptions{},
			func() error {
				return deleteRepoImageInImagesRepo(m.ImagesRepo, m.DryRun, nonexistentGitBranchRepoImages...)
			},
		); err != nil {
			return nil, err
		}

		repoImages = exceptRepoImageList(repoImages, nonexistentGitBranchRepoImages...)
	}

	if len(nonexistentGitCommitRepoImages) != 0 {
		if err := logboek.Default.LogBlock(
			"Removed tags by nonexistent git-commit policy",
			logboek.LevelLogBlockOptions{},
			func() error {
				return deleteRepoImageInImagesRepo(m.ImagesRepo, m.DryRun, nonexistentGitCommitRepoImages...)
			},
		); err != nil {
			return nil, err
		}

		repoImages = exceptRepoImageList(repoImages, nonexistentGitCommitRepoImages...)
	}

	return repoImages, nil
}

func repoImageMetaTagMatch(imageMetaTag string, matches ...string) bool {
	for _, match := range matches {
		if imageMetaTag == slug.DockerTag(match) {
			return true
		}
	}

	return false
}

func (m *imagesCleanupManager) repoImagesCleanupByPolicies(repoImages []*image.Info) ([]*image.Info, error) {
	var repoImagesWithGitTagScheme, repoImagesWithGitCommitScheme, repoImagesWithStagesSignatureScheme []*image.Info

	for _, repoImage := range repoImages {
		strategy, ok := repoImage.Labels[image.WerfTagStrategyLabel]
		if !ok {
			continue
		}

		switch strategy {
		case string(tag_strategy.GitTag):
			repoImagesWithGitTagScheme = append(repoImagesWithGitTagScheme, repoImage)
		case string(tag_strategy.GitCommit):
			repoImagesWithGitCommitScheme = append(repoImagesWithGitCommitScheme, repoImage)
		case string(tag_strategy.StagesSignature):
			repoImagesWithStagesSignatureScheme = append(repoImagesWithStagesSignatureScheme, repoImage)
		}
	}

	cleanupByPolicyOptions := repoImagesCleanupByPolicyOptions{
		hasLimit:        m.Policies.GitTagStrategyHasLimit,
		limit:           m.Policies.GitTagStrategyLimit,
		hasExpiryPeriod: m.Policies.GitTagStrategyHasExpiryPeriod,
		expiryPeriod:    m.Policies.GitTagStrategyExpiryPeriod,
		schemeName:      string(tag_strategy.GitTag),
	}

	var err error
	repoImages, err = m.repoImagesCleanupByPolicy(repoImages, repoImagesWithGitTagScheme, cleanupByPolicyOptions)
	if err != nil {
		return nil, err
	}

	cleanupByPolicyOptions = repoImagesCleanupByPolicyOptions{
		hasLimit:        m.Policies.GitCommitStrategyHasLimit,
		limit:           m.Policies.GitCommitStrategyLimit,
		hasExpiryPeriod: m.Policies.GitCommitStrategyHasExpiryPeriod,
		expiryPeriod:    m.Policies.GitCommitStrategyExpiryPeriod,
		schemeName:      string(tag_strategy.GitCommit),
	}

	repoImages, err = m.repoImagesCleanupByPolicy(repoImages, repoImagesWithGitCommitScheme, cleanupByPolicyOptions)
	if err != nil {
		return nil, err
	}

	cleanupByPolicyOptions = repoImagesCleanupByPolicyOptions{
		hasLimit:        m.Policies.StagesSignatureStrategyHasLimit,
		limit:           m.Policies.StagesSignatureStrategyLimit,
		hasExpiryPeriod: m.Policies.StagesSignatureStrategyHasExpiryPeriod,
		expiryPeriod:    m.Policies.StagesSignatureStrategyExpiryPeriod,
		schemeName:      string(tag_strategy.StagesSignature),
	}

	repoImages, err = m.repoImagesCleanupByPolicy(repoImages, repoImagesWithStagesSignatureScheme, cleanupByPolicyOptions)
	if err != nil {
		return nil, err
	}

	return repoImages, nil
}

type repoImagesCleanupByPolicyOptions struct {
	hasLimit        bool
	limit           int64
	hasExpiryPeriod bool
	expiryPeriod    time.Duration
	schemeName      string
}

func (m *imagesCleanupManager) repoImagesCleanupByPolicy(repoImages, repoImagesWithScheme []*image.Info, options repoImagesCleanupByPolicyOptions) ([]*image.Info, error) {
	var expiryTime time.Time
	if options.hasExpiryPeriod {
		expiryTime = time.Now().Add(-options.expiryPeriod)
	}

	sort.Slice(repoImagesWithScheme, func(i, j int) bool {
		iCreated := repoImagesWithScheme[i].GetCreatedAt()
		jCreated := repoImagesWithScheme[j].GetCreatedAt()
		return iCreated.Before(jCreated)
	})

	var notExpiredRepoImages, expiredRepoImages []*image.Info
	for _, repoImage := range repoImagesWithScheme {
		if options.hasExpiryPeriod && repoImage.GetCreatedAt().Before(expiryTime) {
			expiredRepoImages = append(expiredRepoImages, repoImage)
		} else {
			notExpiredRepoImages = append(notExpiredRepoImages, repoImage)
		}
	}

	if len(expiredRepoImages) != 0 {
		logBlockMessage := fmt.Sprintf("Removed tags by %s date policy (created before %s)", options.schemeName, expiryTime.Format("2006-01-02T15:04:05-0700"))
		if err := logboek.Default.LogBlock(
			logBlockMessage,
			logboek.LevelLogBlockOptions{},
			func() error {
				return deleteRepoImageInImagesRepo(m.ImagesRepo, m.DryRun, expiredRepoImages...)
			},
		); err != nil {
			return nil, err
		}

		repoImages = exceptRepoImageList(repoImages, expiredRepoImages...)
	}

	if options.hasLimit && int64(len(notExpiredRepoImages)) > options.limit {
		excessImagesByLimit := notExpiredRepoImages[:int64(len(notExpiredRepoImages))-options.limit]

		logBlockMessage := fmt.Sprintf("Removed tags by %s limit policy (> %d)", options.schemeName, options.limit)
		if err := logboek.Default.LogBlock(
			logBlockMessage,
			logboek.LevelLogBlockOptions{},
			func() error {
				return deleteRepoImageInImagesRepo(m.ImagesRepo, m.DryRun, excessImagesByLimit...)
			},
		); err != nil {
			return nil, err
		}

		repoImages = exceptRepoImageList(repoImages, excessImagesByLimit...)
	}

	return repoImages, nil
}

func (m *imagesCleanupManager) getImageUnusedCommitHashes(resultImageRepoImageList map[string][]*image.Info) (map[string][]plumbing.Hash, error) {
	unusedImageCommitHashes := map[string][]plumbing.Hash{}

	for imageName, commitHashImageMetadata := range m.getImageCommitHashImageMetadata() {
		var unusedCommitHashes []plumbing.Hash
		repoImageList, ok := resultImageRepoImageList[imageName]
		if !ok {
			repoImageList = []*image.Info{}
		}

	outerLoop:
		for commitHash, imageMetadata := range commitHashImageMetadata {
			for _, repoImage := range repoImageList {
				if repoImage.Labels[image.WerfContentSignatureLabel] == imageMetadata.ContentSignature {
					continue outerLoop
				}
			}

			unusedCommitHashes = append(unusedCommitHashes, commitHash)
		}

		unusedImageCommitHashes[imageName] = unusedCommitHashes
	}

	return unusedImageCommitHashes, nil
}

func (m *imagesCleanupManager) getImageRepoImageListWithoutRelatedImageMetadata(imageRepoImageListToCleanup map[string][]*image.Info, imageContentSignatureRepoImageListToCleanup map[string]map[string][]*image.Info) (map[string][]*image.Info, error) {
	imageRepoImageListWithoutRelatedCommit := map[string][]*image.Info{}

	for imageName, repoImageListToCleanup := range imageRepoImageListToCleanup {
		unusedRepoImageList := repoImageListToCleanup

		contentSignatureRepoImageListToCleanup, ok := imageContentSignatureRepoImageListToCleanup[imageName]
		if !ok {
			contentSignatureRepoImageListToCleanup = map[string][]*image.Info{}
		}

		for _, filteredRepoImageListToCleanup := range contentSignatureRepoImageListToCleanup {
			unusedRepoImageList = exceptRepoImageList(unusedRepoImageList, filteredRepoImageListToCleanup...)
		}

		imageRepoImageListWithoutRelatedCommit[imageName] = unusedRepoImageList
	}

	return imageRepoImageListWithoutRelatedCommit, nil
}

// getImageContentSignatureExistingCommitHashes groups images, content signatures and commit hashes which exist in the git repo.
func (m *imagesCleanupManager) getImageContentSignatureExistingCommitHashes() (map[string]map[string][]plumbing.Hash, error) {
	imageContentSignatureCommitHashes := map[string]map[string][]plumbing.Hash{}

	for _, imageName := range m.ImageNameList {
		imageContentSignatureCommitHashes[imageName] = map[string][]plumbing.Hash{}

		for commitHash, imageMetadata := range m.getImageCommitHashImageMetadata()[imageName] {
			commitHashes, ok := imageContentSignatureCommitHashes[imageName][imageMetadata.ContentSignature]
			if !ok {
				commitHashes = []plumbing.Hash{}
			}

			exist, err := m.LocalGit.IsCommitExists(commitHash.String())
			if err != nil {
				return nil, fmt.Errorf("check git commit existence failed: %s", err)
			}

			if exist {
				commitHashes = append(commitHashes, commitHash)
				imageContentSignatureCommitHashes[imageName][imageMetadata.ContentSignature] = commitHashes
			}
		}
	}

	return imageContentSignatureCommitHashes, nil
}

func deployedDockerImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var deployedDockerImages []string

	images, err := getPodsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get Pods images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getReplicationControllersImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get ReplicationControllers images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getDeploymentsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get Deployments images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getStatefulSetsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get StatefulSets images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getDaemonSetsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get DaemonSets images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getReplicaSetsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get ReplicaSets images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getCronJobsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get CronJobs images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	images, err = getJobsImages(kubernetesClient, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("cannot get Jobs images: %s", err)
	}

	deployedDockerImages = append(deployedDockerImages, images...)

	return deployedDockerImages, nil
}

func getPodsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.CoreV1().Pods(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pod := range list.Items {
		for _, container := range pod.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getReplicationControllersImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.CoreV1().ReplicationControllers(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, replicationController := range list.Items {
		for _, container := range replicationController.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getDeploymentsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.AppsV1().Deployments(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, deployment := range list.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getStatefulSetsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.AppsV1().StatefulSets(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, statefulSet := range list.Items {
		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getDaemonSetsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.AppsV1().DaemonSets(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, daemonSets := range list.Items {
		for _, container := range daemonSets.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getReplicaSetsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.AppsV1().ReplicaSets(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, replicaSet := range list.Items {
		for _, container := range replicaSet.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getCronJobsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.BatchV1beta1().CronJobs(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cronJob := range list.Items {
		for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

func getJobsImages(kubernetesClient kubernetes.Interface, kubernetesNamespace string) ([]string, error) {
	var images []string
	list, err := kubernetesClient.BatchV1().Jobs(kubernetesNamespace).List(v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, job := range list.Items {
		for _, container := range job.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}
