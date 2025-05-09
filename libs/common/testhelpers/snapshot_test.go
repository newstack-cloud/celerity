package testhelpers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SnapshotTestSuite struct {
	suite.Suite
}

func (s *SnapshotTestSuite) Test_snapshot_produces_file_with_safe_name() {
	// This test is not a real test, but a sanity check to ensure
	// that the snapshot name is safe to use as a file name on all
	// platforms. It will not fail if the snapshot name is not safe,
	// but it will fail if the snapshot name is not what we expect.
	err := Snapshot("test-value-for-snapshot")
	s.Require().NoError(err)

	snapshotFiles, err := os.ReadDir(".snapshots")
	s.Require().NoError(err)

	s.Assert().Len(snapshotFiles, 1)
	snapshotFile := snapshotFiles[0]
	snapshotFileName := snapshotFile.Name()
	s.Assert().Equal(
		"testhelpers-(SnapshotTestSuite)-Test_snapshot_produces_file_with_safe_name",
		snapshotFileName,
	)
}

func TestSnapshotTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotTestSuite))
}
