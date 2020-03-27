package build

import (
	"fmt"

	"github.com/flant/werf/pkg/container_runtime"

	"github.com/google/uuid"

	"github.com/flant/logboek"

	"github.com/flant/werf/pkg/build/stage"
	"github.com/flant/werf/pkg/image"
	"github.com/flant/werf/pkg/util"
)

func NewPrepareStagesPhase(c *Conveyor) *PrepareStagesPhase {
	return &PrepareStagesPhase{
		BasePhase: BasePhase{c},
	}
}

type PrepareStagesPhase struct {
	BasePhase
	StagesIterator *StagesIterator
}

func (phase *PrepareStagesPhase) Name() string {
	return "prepareStages"
}

func (phase *PrepareStagesPhase) BeforeImages() error {
	return nil
}

func (phase *PrepareStagesPhase) AfterImages() error {
	return nil
}

func (phase *PrepareStagesPhase) ImageProcessingShouldBeStopped(img *Image) bool {
	return false
}

func (phase *PrepareStagesPhase) BeforeImageStages(img *Image) error {
	phase.StagesIterator = NewStagesIterator(phase.Conveyor)

	img.SetupBaseImage(phase.Conveyor)

	return nil
}

func (phase *PrepareStagesPhase) AfterImageStages(img *Image) error {
	img.SetLastNonEmptyStage(phase.StagesIterator.PrevNonEmptyStage)

	stagesSig, err := calculateSignature("imageStages", "", phase.StagesIterator.PrevNonEmptyStage, phase.Conveyor)
	if err != nil {
		return fmt.Errorf("unable to calculate image %s stages-signature: %s", img.GetName(), err)
	}
	img.SetStagesSignature(stagesSig)

	return nil
}

func (phase *PrepareStagesPhase) OnImageStage(img *Image, stg stage.Interface) error {
	return phase.StagesIterator.OnImageStage(img, stg, func(img *Image, stg stage.Interface, isEmpty bool) error {
		return phase.onImageStage(img, stg, isEmpty)
	})
}

func calculateSignature(stageName, stageDependencies string, prevNonEmptyStage stage.Interface, conveyor *Conveyor) (string, error) {
	checksumArgs := []string{image.BuildCacheVersion, stageName, stageDependencies}
	if prevNonEmptyStage != nil {
		prevStageDependencies, err := prevNonEmptyStage.GetNextStageDependencies(conveyor)
		if err != nil {
			return "", fmt.Errorf("unable to get prev stage %s dependencies for the stage %s: %s", prevNonEmptyStage.Name(), stageName, err)
		}

		checksumArgs = append(checksumArgs, prevNonEmptyStage.GetSignature(), prevStageDependencies)
	}

	signature := util.Sha3_224Hash(checksumArgs...)

	blockMsg := fmt.Sprintf("Stage %s signature %s", stageName, signature)
	_ = logboek.Debug.LogBlock(blockMsg, logboek.LevelLogBlockOptions{}, func() error {
		checksumArgsNames := []string{
			"BuildCacheVersion",
			"stageName",
			"stageDependencies",
			"prevNonEmptyStage signature",
			"prevNonEmptyStage dependencies for next stage",
		}
		for ind, checksumArg := range checksumArgs {
			logboek.Debug.LogF("%s => %q\n", checksumArgsNames[ind], checksumArg)
		}
		return nil
	})

	return signature, nil
}

func (phase *PrepareStagesPhase) setStageLinks(img *Image, stg stage.Interface, isEmpty bool) {
	if phase.StagesIterator.PrevStage != nil {
		phase.StagesIterator.PrevStage.SetNextStage(stg)
		stg.SetPrevStage(phase.StagesIterator.PrevStage)
	}
	if phase.StagesIterator.PrevBuiltStage != nil {
		stg.SetPrevBuiltStage(phase.StagesIterator.PrevBuiltStage)
	}
	if phase.StagesIterator.PrevNonEmptyStage != nil {
		stg.SetPrevNonEmptyStage(phase.StagesIterator.PrevNonEmptyStage)
	}
	if prevImage := phase.StagesIterator.GetPrevImage(img, stg); prevImage != nil {
		stg.SetPrevImage(prevImage)
	}
	if prevBuiltImage := phase.StagesIterator.GetPrevBuiltImage(img, stg); prevBuiltImage != nil {
		stg.SetPrevBuiltImage(prevBuiltImage)
	}

	if !isEmpty {
		prevStage := stg.GetPrevStage()
		for prevStage != nil {
			prevStage.SetNextNonEmptyStage(stg)
			// Links before previous non empty stage should already be set, so exit
			if stg.GetPrevNonEmptyStage() == prevStage {
				break
			}
			prevStage = prevStage.GetPrevStage()
		}
	}
}

func (phase *PrepareStagesPhase) onImageStage(img *Image, stg stage.Interface, isEmpty bool) error {
	phase.setStageLinks(img, stg, isEmpty)

	if isEmpty {
		return nil
	}

	stageDependencies, err := stg.GetDependencies(phase.Conveyor, phase.StagesIterator.GetPrevImage(img, stg), phase.StagesIterator.GetPrevBuiltImage(img, stg))
	if err != nil {
		return err
	}

	stageSig, err := calculateSignature(string(stg.Name()), stageDependencies, phase.StagesIterator.PrevNonEmptyStage, phase.Conveyor)
	if err != nil {
		return err
	}
	stg.SetSignature(stageSig)

	var i *container_runtime.StageImage
	var suitableImageFound bool

	cacheExists, cacheImagesDescs, err := getImagesBySignatureFromCache(phase.Conveyor, string(stg.Name()), stageSig)
	if err != nil {
		return err
	}

	if cacheExists {
		if imgInfo, err := selectSuitableStagesStorageImage(stg, cacheImagesDescs); err != nil {
			return err
		} else if imgInfo != nil {
			suitableImageFound = true
			i = phase.Conveyor.GetOrCreateStageImage(phase.StagesIterator.GetPrevImage(img, stg).(*container_runtime.StageImage), imgInfo.Name)
			i.SetStagesStorageImageInfo(imgInfo)
			stg.SetImage(i)
		}
	} else {
		logboek.Info.LogF(
			"Stage %q cache by signature %s is not exists in stages storage cache: resetting stages storage cache\n",
			stg.Name(), stageSig,
		)

		imagesDescs, err := atomicGetImagesBySignatureFromStagesStorageWithCacheReset(phase.Conveyor, stg.LogDetailedName(), stageSig)
		if err != nil {
			return err
		}

		if imgInfo, err := selectSuitableStagesStorageImage(stg, imagesDescs); err != nil {
			return err
		} else if imgInfo != nil {
			suitableImageFound = true

			i = phase.Conveyor.GetOrCreateStageImage(phase.StagesIterator.GetPrevImage(img, stg).(*container_runtime.StageImage), imgInfo.Name)
			i.SetStagesStorageImageInfo(imgInfo)
			stg.SetImage(i)
		}
	}

	if !suitableImageFound {
		// Will build a new image
		i = phase.Conveyor.GetOrCreateStageImage(phase.StagesIterator.GetPrevImage(img, stg).(*container_runtime.StageImage), uuid.New().String())
		stg.SetImage(i)
	}

	if err = stg.AfterSignatureCalculated(phase.Conveyor); err != nil {
		return err
	}

	return nil
}
