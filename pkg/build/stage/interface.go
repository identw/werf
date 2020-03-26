package stage

import (
	"github.com/flant/werf/pkg/container_runtime"
	"github.com/flant/werf/pkg/image"
)

type Interface interface {
	Name() StageName
	LogDetailedName() string

	IsEmpty(c Conveyor, prevBuiltImage container_runtime.ImageInterface) (bool, error)
	ShouldBeReset(builtImage container_runtime.ImageInterface) (bool, error)

	GetDependencies(c Conveyor, prevImage container_runtime.ImageInterface, prevBuiltImage container_runtime.ImageInterface) (string, error)
	GetNextStageDependencies(c Conveyor) (string, error)

	PrepareImage(c Conveyor, prevBuiltImage, image container_runtime.ImageInterface) error

	AfterSignatureCalculated(Conveyor) error
	PreRunHook(Conveyor) error

	SetSignature(signature string)
	GetSignature() string

	SetImage(container_runtime.ImageInterface)
	GetImage() container_runtime.ImageInterface

	SetGitMappings([]*GitMapping)
	GetGitMappings() []*GitMapping

	SelectCacheImage(images []*image.Info) (*image.Info, error)

	GetPrevStage() Interface
	GetPrevBuiltStage() Interface
	GetPrevNonEmptyStage() Interface
	GetPrevImage() container_runtime.ImageInterface
	GetPrevBuiltImage() container_runtime.ImageInterface
	GetNextStage() Interface
	GetNextNonEmptyStage() Interface

	SetPrevStage(stage Interface)
	SetPrevBuiltStage(stage Interface)
	SetPrevNonEmptyStage(stage Interface)
	SetPrevImage(image container_runtime.ImageInterface)
	SetPrevBuiltImage(image container_runtime.ImageInterface)
	SetNextStage(stage Interface)
	SetNextNonEmptyStage(stage Interface)
}
