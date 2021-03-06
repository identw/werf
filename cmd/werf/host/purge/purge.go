package reset

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/flant/logboek"
	"github.com/flant/shluz"

	"github.com/flant/werf/cmd/werf/common"
	"github.com/flant/werf/pkg/cleaning"
	"github.com/flant/werf/pkg/docker"
	"github.com/flant/werf/pkg/docker_registry"
	"github.com/flant/werf/pkg/werf"
)

var cmdData struct {
	Force bool
}

var commonCmdData common.CmdData

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Purge werf images, stages, cache and other data of all projects on host machine",
		Long: common.GetLongCommandDescription(`Purge werf images, stages, cache and other data of all projects on host machine.

The data include:
* Old service tmp dirs, which werf creates during every build, publish, deploy and other commands.
* Local cache:
  * Remote git clones cache.
  * Git worktree cache.
* Shared context:
  * Mounts which persists between several builds (mounts from build_dir).

WARNING: Do not run this command during any other werf command is working on the host machine. This command is supposed to be run manually.`),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := common.ProcessLogOptions(&commonCmdData); err != nil {
				common.PrintHelp(cmd)
				return err
			}
			common.LogVersion()

			return common.LogRunningTime(func() error {
				return runReset()
			})
		},
	}

	common.SetupTmpDir(&commonCmdData, cmd)
	common.SetupHomeDir(&commonCmdData, cmd)
	common.SetupDockerConfig(&commonCmdData, cmd, "")
	common.SetupInsecureRegistry(&commonCmdData, cmd)
	common.SetupSkipTlsVerifyRegistry(&commonCmdData, cmd)

	common.SetupLogOptions(&commonCmdData, cmd)

	common.SetupDryRun(&commonCmdData, cmd)
	cmd.Flags().BoolVarP(&cmdData.Force, "force", "", false, common.CleaningCommandsForceOptionDescription)

	return cmd
}

func runReset() error {
	if err := werf.Init(*commonCmdData.TmpDir, *commonCmdData.HomeDir); err != nil {
		return fmt.Errorf("initialization error: %s", err)
	}

	if err := shluz.Init(filepath.Join(werf.GetServiceDir(), "locks")); err != nil {
		return err
	}

	if err := docker_registry.Init(docker_registry.Options{InsecureRegistry: *commonCmdData.InsecureRegistry, SkipTlsVerifyRegistry: *commonCmdData.SkipTlsVerifyRegistry}); err != nil {
		return err
	}

	if err := docker.Init(*commonCmdData.DockerConfig, *commonCmdData.LogVerbose, *commonCmdData.LogDebug); err != nil {
		return err
	}

	logboek.LogOptionalLn()
	hostPurgeOptions := cleaning.HostPurgeOptions{DryRun: *commonCmdData.DryRun, RmContainersThatUseWerfImages: cmdData.Force}
	if err := cleaning.HostPurge(hostPurgeOptions); err != nil {
		return err
	}

	return nil
}
