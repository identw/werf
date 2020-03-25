package build

import (
	"fmt"

	"github.com/flant/logboek"
	"github.com/flant/werf/pkg/build/stage"
	"github.com/flant/werf/pkg/container_runtime"
	"github.com/flant/werf/pkg/storage"
)

func fetchStage(stagesStorage storage.StagesStorage, stg stage.Interface) error {
	if shouldFetch, err := stagesStorage.ShouldFetchImage(&container_runtime.DockerImage{Image: stg.GetImage()}); err == nil && shouldFetch {
		if err := logboek.Default.LogProcess(
			fmt.Sprintf("Fetching stage %s from stages storage", stg.LogDetailedName()),
			logboek.LevelLogProcessOptions{Style: logboek.HighlightStyle()},
			func() error {
				logboek.Info.LogF("Image name: %s\n", stg.GetImage().Name())
				if err := stagesStorage.FetchImage(&container_runtime.DockerImage{Image: stg.GetImage()}); err != nil {
					return fmt.Errorf("unable to fetch stage %s image %s from stages storage %s: %s", stg.LogDetailedName(), stg.GetImage().Name(), stagesStorage.String(), err)
				}
				return nil
			},
		); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}
