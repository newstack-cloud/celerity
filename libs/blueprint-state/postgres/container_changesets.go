package postgres

import (
	"context"
	"time"

	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
)

type changesetsContainerImpl struct{}

func (c *changesetsContainerImpl) Save(
	ctx context.Context,
	changeset *manage.Changeset,
) error {
	return nil
}

func (c *changesetsContainerImpl) Get(
	ctx context.Context,
	id string,
) (*manage.Changeset, error) {
	return nil, nil
}

func (c *changesetsContainerImpl) Cleanup(
	ctx context.Context,
	thresholdDate time.Time,
) error {
	return nil
}
