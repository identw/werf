package cleanup

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	"github.com/werf/kubedog/pkg/kube"
	"github.com/werf/logboek"

	"github.com/werf/werf/cmd/werf/common"
	"github.com/werf/werf/pkg/cleaning"
	"github.com/werf/werf/pkg/container_runtime"
	"github.com/werf/werf/pkg/docker"
	"github.com/werf/werf/pkg/image"
	"github.com/werf/werf/pkg/stages_manager"
	"github.com/werf/werf/pkg/tmp_manager"
	"github.com/werf/werf/pkg/true_git"
	"github.com/werf/werf/pkg/werf"
)

var commonCmdData common.CmdData

var cmdData struct {
	GitHistoryBasedCleanup    bool
	GitHistoryBasedCleanupV12 bool
	CheckAllNamespaces        bool
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "cleanup",
		DisableFlagsInUseLine: true,
		Short:                 "Safely cleanup unused project images and stages",
		Long: common.GetLongCommandDescription(`Safely cleanup unused project images and stages.

First step is 'werf images cleanup' command, which will delete unused images from images repo. Second step is 'werf stages cleanup' command, which will delete unused stages from stages storage to be in sync with the images repo.

It is safe to run this command periodically (daily is enough) by automated cleanup job in parallel with other werf commands such as build, deploy and host cleanup.`),
		Example: `  $ werf cleanup --stages-storage :local --images-repo registry.mydomain.com/myproject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer werf.PrintGlobalWarnings()

			if err := common.ProcessLogOptions(&commonCmdData); err != nil {
				common.PrintHelp(cmd)
				return err
			}
			common.LogVersion()

			return common.LogRunningTime(func() error {
				return runCleanup()
			})
		},
	}

	common.SetupDir(&commonCmdData, cmd)
	common.SetupConfigPath(&commonCmdData, cmd)
	common.SetupConfigTemplatesDir(&commonCmdData, cmd)
	common.SetupTmpDir(&commonCmdData, cmd)
	common.SetupHomeDir(&commonCmdData, cmd)

	common.SetupStagesStorageOptions(&commonCmdData, cmd)
	common.SetupImagesRepoOptions(&commonCmdData, cmd)

	common.SetupDockerConfig(&commonCmdData, cmd, "Command needs granted permissions to read, pull and delete images from the specified stages storage and images repo")
	common.SetupInsecureRegistry(&commonCmdData, cmd)
	common.SetupSkipTlsVerifyRegistry(&commonCmdData, cmd)
	common.SetupImagesCleanupPolicies(&commonCmdData, cmd)

	common.SetupGitHistorySynchronization(&commonCmdData, cmd)
	common.SetupAllowGitShallowClone(&commonCmdData, cmd)

	cmd.Flags().BoolVarP(&cmdData.GitHistoryBasedCleanup, "git-history-based-cleanup", "", common.GetBoolEnvironmentDefaultFalse("WERF_GIT_HISTORY_BASED_CLEANUP"), "Use git history based cleanup (default $WERF_GIT_HISTORY_BASED_CLEANUP)")
	cmd.Flags().BoolVarP(&cmdData.GitHistoryBasedCleanupV12, "git-history-based-cleanup-v1.2", "", common.GetBoolEnvironmentDefaultFalse("WERF_GIT_HISTORY_BASED_CLEANUP_v1_2"), "Use git history based cleanup and delete images tags without related image metadata (default $WERF_GIT_HISTORY_BASED_CLEANUP_v1_2)")
	cmd.Flags().BoolVarP(&cmdData.CheckAllNamespaces, "check-all-namespaces", "", common.GetBoolEnvironmentDefaultTrue("WERF_CHECK_ALL_NAMESPACES"), "Check images in all namespaces (default $WERF_CHECK_ALL_NAMESPACES)")
	common.SetupDryRun(&commonCmdData, cmd)

	common.SetupLogOptions(&commonCmdData, cmd)
	common.SetupLogProjectDir(&commonCmdData, cmd)

	common.SetupSynchronization(&commonCmdData, cmd)
	common.SetupKubeConfig(&commonCmdData, cmd)
	common.SetupKubeConfigBase64(&commonCmdData, cmd)
	common.SetupKubeContext(&commonCmdData, cmd)
	common.SetupWithoutKube(&commonCmdData, cmd)

	return cmd
}

func runCleanup() error {
	if err := werf.Init(*commonCmdData.TmpDir, *commonCmdData.HomeDir); err != nil {
		return fmt.Errorf("initialization error: %s", err)
	}

	if err := true_git.Init(true_git.Options{Out: logboek.GetOutStream(), Err: logboek.GetErrStream(), LiveGitOutput: *commonCmdData.LogVerbose || *commonCmdData.LogDebug}); err != nil {
		return err
	}

	if err := image.Init(); err != nil {
		return err
	}

	if err := common.DockerRegistryInit(&commonCmdData); err != nil {
		return err
	}

	if err := docker.Init(*commonCmdData.DockerConfig, *commonCmdData.LogVerbose, *commonCmdData.LogDebug); err != nil {
		return err
	}

	if err := kube.Init(kube.InitOptions{KubeConfigOptions: kube.KubeConfigOptions{
		Context:          *commonCmdData.KubeContext,
		ConfigPath:       *commonCmdData.KubeConfig,
		ConfigDataBase64: *commonCmdData.KubeConfigBase64,
	}}); err != nil {
		return fmt.Errorf("cannot initialize kube: %s", err)
	}

	if err := common.InitKubedog(); err != nil {
		return fmt.Errorf("cannot init kubedog: %s", err)
	}

	projectDir, err := common.GetProjectDir(&commonCmdData)
	if err != nil {
		return fmt.Errorf("getting project dir failed: %s", err)
	}

	common.ProcessLogProjectDir(&commonCmdData, projectDir)

	projectTmpDir, err := tmp_manager.CreateProjectDir()
	if err != nil {
		return fmt.Errorf("getting project tmp dir failed: %s", err)
	}
	defer tmp_manager.ReleaseProjectDir(projectTmpDir)

	werfConfig, err := common.GetRequiredWerfConfig(projectDir, &commonCmdData, true)
	if err != nil {
		return fmt.Errorf("unable to load werf config: %s", err)
	}

	projectName := werfConfig.Meta.Project

	containerRuntime := &container_runtime.LocalDockerServerRuntime{} // TODO

	stagesStorage, err := common.GetStagesStorage(containerRuntime, &commonCmdData)
	if err != nil {
		return err
	}

	synchronization, err := common.GetSynchronization(&commonCmdData, projectName, stagesStorage)
	if err != nil {
		return err
	}
	stagesStorageCache, err := common.GetStagesStorageCache(synchronization)
	if err != nil {
		return err
	}
	storageLockManager, err := common.GetStorageLockManager(synchronization)
	if err != nil {
		return err
	}

	stagesManager := stages_manager.NewStagesManager(projectName, storageLockManager, stagesStorageCache)
	if err := stagesManager.UseStagesStorage(stagesStorage); err != nil {
		return err
	}

	imagesRepo, err := common.GetImagesRepo(projectName, &commonCmdData)
	if err != nil {
		return err
	}

	imagesNames, err := common.GetManagedImagesNames(projectName, stagesStorage, werfConfig)
	if err != nil {
		return err
	}
	logboek.Debug.LogF("Managed images names: %v\n", imagesNames)

	localGitRepo, err := common.GetLocalGitRepoForImagesCleanup(projectDir, &commonCmdData)
	if err != nil {
		return err
	}

	policies, err := common.GetImagesCleanupPolicies(&commonCmdData)
	if err != nil {
		return err
	}

	kubernetesContextsClients := map[string]kubernetes.Interface{}
	if *commonCmdData.KubeContext != "" {
		kubernetesContextsClients[*commonCmdData.KubeContext] = kube.Client
	} else {
		kubernetesContextsClients, err = kube.GetAllContextsClients(kube.GetAllContextsClientsOptions{KubeConfig: *commonCmdData.KubeConfig})
		if err != nil {
			return fmt.Errorf("unable to get Kubernetes clusters connections: %s", err)
		}
	}

	kubernetesNamespaces := map[string]string{}
	clientConfig, _ := kube.GetClientConfig("", *commonCmdData.KubeConfig, nil)
	rc, _ := clientConfig.RawConfig()
	for contextName, context := range rc.Contexts {
		if cmdData.CheckAllNamespaces {
			// "" - cluster scope, therefore all namespaces
			kubernetesNamespaces[contextName] = ""
		} else {
			kubernetesNamespaces[contextName] = context.Namespace
		}
	}

	cleanupOptions := cleaning.CleanupOptions{
		ImagesCleanupOptions: cleaning.ImagesCleanupOptions{
			ImageNameList:                 imagesNames,
			LocalGit:                      localGitRepo,
			KubernetesContextsClients:     kubernetesContextsClients,
			KubernetesNamespaces:          kubernetesNamespaces,
			WithoutKube:                   *commonCmdData.WithoutKube,
			Policies:                      policies,
			GitHistoryBasedCleanup:        cmdData.GitHistoryBasedCleanup,
			GitHistoryBasedCleanupV12:     cmdData.GitHistoryBasedCleanupV12,
			GitHistoryBasedCleanupOptions: werfConfig.Meta.Cleanup,
			DryRun:                        *commonCmdData.DryRun,
		},
		StagesCleanupOptions: cleaning.StagesCleanupOptions{
			ImageNameList: imagesNames,
			DryRun:        *commonCmdData.DryRun,
		},
	}

	logboek.LogOptionalLn()
	if err := cleaning.Cleanup(projectName, imagesRepo, storageLockManager, stagesManager, cleanupOptions); err != nil {
		return err
	}

	return nil
}
