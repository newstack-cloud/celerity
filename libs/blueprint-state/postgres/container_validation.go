package postgres

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
)

type validationContainerImpl struct{}

func (v *validationContainerImpl) Save(
	ctx context.Context,
	validation *manage.BlueprintValidation,
) error {
	return nil
}

func (v *validationContainerImpl) Get(
	ctx context.Context,
	id string,
) (*manage.BlueprintValidation, error) {
	return nil, nil
}

func (v *validationContainerImpl) Cleanup(
	ctx context.Context,
	thresholdDate time.Time,
) error {
	return nil
}
