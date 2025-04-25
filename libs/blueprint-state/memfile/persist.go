package memfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"sync"

	"github.com/spf13/afero"
	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

const (
	malformedInstanceStateFileMessage      = "instance state file is malformed"
	malformedResourceDriftStateFileMessage = "resource drift state file is malformed"
)

type statePersister struct {
	stateDir               string
	fs                     afero.Fs
	instanceIndex          map[string]*indexLocation
	lastInstanceChunk      int
	maxGuideFileSize       int64
	resourceDriftIndex     map[string]*indexLocation
	lastResourceDriftChunk int
	eventIndex             map[string]*eventIndexLocation
	maxEventPartitionSize  int64
	// The persister has its own mutex, separate from
	// the state container's read/write lock.
	mu sync.Mutex
}

func (s *statePersister) createInstance(instance *state.InstanceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastChunkFilePath := instanceChunkFilePath(s.stateDir, s.lastInstanceChunk)
	chunkFileInfo, err := s.fs.Stat(lastChunkFilePath)
	if err != nil {
		return err
	}

	chunkFilePath, err := s.prepareInstanceChunkFile(chunkFileInfo, lastChunkFilePath)
	if err != nil {
		return err
	}

	existingData, err := afero.ReadFile(s.fs, chunkFilePath)
	if err != nil {
		return err
	}

	chunkInstances := []*persistedInstanceState{}
	err = json.Unmarshal(existingData, &chunkInstances)
	if err != nil {
		return err
	}

	chunkInstances = append(chunkInstances, toPersistedInstanceState(instance))

	updatedData, err := json.Marshal(chunkInstances)
	if err != nil {
		return err
	}

	err = afero.WriteFile(s.fs, chunkFilePath, updatedData, 0644)
	if err != nil {
		return err
	}

	return s.addToInstanceIndex(instance, len(chunkInstances)-1)
}

func (s *statePersister) prepareInstanceChunkFile(
	chunkFileInfo os.FileInfo,
	lastChunkFilePath string,
) (string, error) {
	if chunkFileInfo.Size() >= s.maxGuideFileSize {
		s.lastInstanceChunk += 1
		newChunkFilePath := instanceChunkFilePath(s.stateDir, s.lastInstanceChunk)
		err := afero.WriteFile(s.fs, newChunkFilePath, []byte("[]"), 0644)
		if err != nil {
			return "", err
		}

		return newChunkFilePath, nil
	}

	return lastChunkFilePath, nil
}

func (s *statePersister) addToInstanceIndex(instance *state.InstanceState, indexInFile int) error {
	s.instanceIndex[instance.InstanceID] = &indexLocation{
		ChunkNumber:  s.lastInstanceChunk,
		IndexInChunk: indexInFile,
	}

	return s.persistInstanceIndexFile()
}

func (s *statePersister) updateInstance(instance *state.InstanceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := s.loadAndValidateInstanceEntry(instance)
	if err != nil {
		return err
	}

	info.chunkInstances[info.entry.IndexInChunk] = toPersistedInstanceState(instance)

	updatedData, err := json.Marshal(info.chunkInstances)
	if err != nil {
		return err
	}

	return afero.WriteFile(s.fs, info.chunkFilePath, updatedData, 0644)
}

func (s *statePersister) removeInstance(instance *state.InstanceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := s.loadAndValidateInstanceEntry(instance)
	if err != nil {
		return err
	}

	chunkInstances := slices.Delete(
		info.chunkInstances,
		info.entry.IndexInChunk,
		info.entry.IndexInChunk+1,
	)

	updatedData, err := json.Marshal(chunkInstances)
	if err != nil {
		return err
	}

	err = afero.WriteFile(s.fs, info.chunkFilePath, updatedData, 0644)
	if err != nil {
		return err
	}

	return s.removeFromInstanceIndex(instance)
}

type persistedInstanceInfo struct {
	chunkInstances []*persistedInstanceState
	entry          *indexLocation
	chunkFilePath  string
}

// A lock must be held when calling this method.
func (s *statePersister) loadAndValidateInstanceEntry(
	instance *state.InstanceState,
) (*persistedInstanceInfo, error) {
	entry, hasEntry := s.instanceIndex[instance.InstanceID]
	if !hasEntry {
		return nil, state.InstanceNotFoundError(instance.InstanceID)
	}

	chunkFilePath := instanceChunkFilePath(s.stateDir, entry.ChunkNumber)
	existingData, err := afero.ReadFile(s.fs, chunkFilePath)
	if err != nil {
		return nil, err
	}

	chunkInstances := []*persistedInstanceState{}
	err = json.Unmarshal(existingData, &chunkInstances)
	if err != nil {
		return nil, err
	}

	if entry.IndexInChunk == -1 ||
		entry.IndexInChunk >= len(chunkInstances) {
		return nil, errMalformedStateFile(malformedInstanceStateFileMessage)
	}

	return &persistedInstanceInfo{
		chunkInstances: chunkInstances,
		entry:          entry,
		chunkFilePath:  chunkFilePath,
	}, nil
}

func (s *statePersister) removeFromInstanceIndex(instance *state.InstanceState) error {
	delete(s.instanceIndex, instance.InstanceID)
	return s.persistInstanceIndexFile()
}

func (s *statePersister) persistInstanceIndexFile() error {
	indexData, err := json.Marshal(s.instanceIndex)
	if err != nil {
		return err
	}

	indexFilePath := instanceIndexFilePath(s.stateDir)
	return afero.WriteFile(s.fs, indexFilePath, indexData, 0644)
}

func (s *statePersister) createResourceDrift(resourceDrift *state.ResourceDriftState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastChunkFilePath := resourceDriftChunkFilePath(s.stateDir, s.lastResourceDriftChunk)
	chunkFileInfo, err := s.fs.Stat(lastChunkFilePath)
	if err != nil {
		return err
	}

	chunkFilePath, err := s.prepareResourceDriftChunkFile(chunkFileInfo, lastChunkFilePath)
	if err != nil {
		return err
	}

	existingData, err := afero.ReadFile(s.fs, chunkFilePath)
	if err != nil {
		return err
	}

	chunkResourceDriftEntries := []*state.ResourceDriftState{}
	err = json.Unmarshal(existingData, &chunkResourceDriftEntries)
	if err != nil {
		return err
	}

	chunkResourceDriftEntries = append(chunkResourceDriftEntries, resourceDrift)

	updatedData, err := json.Marshal(chunkResourceDriftEntries)
	if err != nil {
		return err
	}

	err = afero.WriteFile(s.fs, chunkFilePath, updatedData, 0644)
	if err != nil {
		return err
	}

	return s.addToResourceDriftIndex(resourceDrift, len(chunkResourceDriftEntries)-1)
}

func (s *statePersister) prepareResourceDriftChunkFile(
	chunkFileInfo os.FileInfo,
	lastChunkFilePath string,
) (string, error) {
	if chunkFileInfo.Size() >= s.maxGuideFileSize {
		s.lastResourceDriftChunk += 1
		newChunkFilePath := resourceDriftChunkFilePath(s.stateDir, s.lastResourceDriftChunk)
		err := afero.WriteFile(s.fs, newChunkFilePath, []byte("[]"), 0644)
		if err != nil {
			return "", err
		}

		return newChunkFilePath, nil
	}

	return lastChunkFilePath, nil
}

func (s *statePersister) addToResourceDriftIndex(resourceDrift *state.ResourceDriftState, indexInFile int) error {
	s.resourceDriftIndex[resourceDrift.ResourceID] = &indexLocation{
		ChunkNumber:  s.lastInstanceChunk,
		IndexInChunk: indexInFile,
	}

	return s.persistResourceDriftIndexFile()
}

func (s *statePersister) persistResourceDriftIndexFile() error {
	indexData, err := json.Marshal(s.resourceDriftIndex)
	if err != nil {
		return err
	}

	indexFilePath := resourceDriftIndexFilePath(s.stateDir)
	return afero.WriteFile(s.fs, indexFilePath, indexData, 0644)
}

func (s *statePersister) updateResourceDrift(resourceDrift *state.ResourceDriftState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := s.loadAndValidateResourceDriftEntry(resourceDrift)
	if err != nil {
		return err
	}

	info.chunkResourceDriftEntries[info.entry.IndexInChunk] = resourceDrift

	updatedData, err := json.Marshal(info.chunkResourceDriftEntries)
	if err != nil {
		return err
	}

	return afero.WriteFile(s.fs, info.chunkFilePath, updatedData, 0644)
}

func (s *statePersister) removeResourceDrift(resourceDrift *state.ResourceDriftState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := s.loadAndValidateResourceDriftEntry(resourceDrift)
	if err != nil {
		return err
	}

	info.chunkResourceDriftEntries = slices.Delete(
		info.chunkResourceDriftEntries,
		info.entry.IndexInChunk,
		info.entry.IndexInChunk+1,
	)

	updatedData, err := json.Marshal(info.chunkResourceDriftEntries)
	if err != nil {
		return err
	}

	err = afero.WriteFile(s.fs, info.chunkFilePath, updatedData, 0644)
	if err != nil {
		return err
	}

	return s.removeFromResourceDriftIndex(resourceDrift)
}

type persistedResourceDriftInfo struct {
	chunkResourceDriftEntries []*state.ResourceDriftState
	entry                     *indexLocation
	chunkFilePath             string
}

// A lock must be held when calling this method.
func (s *statePersister) loadAndValidateResourceDriftEntry(
	resourceDrift *state.ResourceDriftState,
) (*persistedResourceDriftInfo, error) {
	entry, hasEntry := s.resourceDriftIndex[resourceDrift.ResourceID]
	if !hasEntry {
		return nil, state.ResourceNotFoundError(resourceDrift.ResourceID)
	}

	chunkFilePath := resourceDriftChunkFilePath(s.stateDir, entry.ChunkNumber)
	existingData, err := afero.ReadFile(s.fs, chunkFilePath)
	if err != nil {
		return nil, err
	}

	chunkResourceDriftEntries := []*state.ResourceDriftState{}
	err = json.Unmarshal(existingData, &chunkResourceDriftEntries)
	if err != nil {
		return nil, err
	}

	if entry.IndexInChunk == -1 ||
		entry.IndexInChunk >= len(chunkResourceDriftEntries) {
		return nil, errMalformedStateFile(malformedResourceDriftStateFileMessage)
	}

	return &persistedResourceDriftInfo{
		chunkResourceDriftEntries: chunkResourceDriftEntries,
		entry:                     entry,
		chunkFilePath:             chunkFilePath,
	}, nil
}

func (s *statePersister) removeFromResourceDriftIndex(resourceDrift *state.ResourceDriftState) error {
	delete(s.resourceDriftIndex, resourceDrift.ResourceID)
	return s.persistInstanceIndexFile()
}

func (s *statePersister) saveEventPartition(
	partitionName string,
	partition []*manage.Event,
	eventToSave *manage.Event,
	eventPartitionIndex int,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Unlike other entities modelled in the state container,
	// events are stored in partition files, where each channel
	// has its own partition.
	// As it is efficient to look up events by their channel in memory
	// for streaming events, a partition is managed in memory.
	// This means that all we need to do is persist the current in-memory
	// partition to disk.
	partitionFilePath := eventPartitionFilePath(s.stateDir, partitionName)

	partitionData, err := json.Marshal(partition)
	if err != nil {
		return err
	}

	if len(partitionData) > int(s.maxEventPartitionSize) {
		return errMaxEventPartitionSizeExceeded(
			partitionName,
			s.maxEventPartitionSize,
		)
	}

	err = afero.WriteFile(s.fs, partitionFilePath, partitionData, 0644)
	if err != nil {
		return err
	}

	return s.addEventToIndex(
		eventToSave,
		partitionName,
		eventPartitionIndex,
	)
}

func (s *statePersister) updateEventPartitionsForRemovals(
	partitions map[string][]*manage.Event,
	removedPartitions []string,
	removedEvents []string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, partitionName := range removedPartitions {
		partitionFilePath := eventPartitionFilePath(s.stateDir, partitionName)
		partitionFileExists, err := afero.Exists(s.fs, partitionFilePath)
		if err != nil {
			return err
		}

		if partitionFileExists {
			err := s.fs.Remove(partitionFilePath)
			if err != nil {
				return err
			}
		}
	}

	for partitionName, partition := range partitions {
		partitionFilePath := eventPartitionFilePath(s.stateDir, partitionName)

		partitionData, err := json.Marshal(partition)
		if err != nil {
			return err
		}

		err = afero.WriteFile(s.fs, partitionFilePath, partitionData, 0644)
		if err != nil {
			return err
		}
	}

	return s.removeEventIndexEntries(removedEvents)
}

// A lock must be held when calling this method.
func (s *statePersister) removeEventIndexEntries(eventIDs []string) error {
	for _, eventID := range eventIDs {
		delete(s.eventIndex, eventID)
	}

	return s.persistEventIndexFile()
}

// A lock must be held when calling this method.
func (s *statePersister) addEventToIndex(
	event *manage.Event,
	partitionName string,
	indexInPartition int,
) error {
	s.eventIndex[event.ID] = &eventIndexLocation{
		Partition:        toPartitionFileBaseName(partitionName),
		IndexInPartition: indexInPartition,
	}

	return s.persistEventIndexFile()
}

// Reads from the event index must be done through this method to ensure
// that the mutex is held when reading from the index.
// This must only be called for reading in contexts that are not holding
// a lock on the state persister.
func (s *statePersister) getEventIndexEntry(eventID string) *eventIndexLocation {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, hasEntry := s.eventIndex[eventID]
	if !hasEntry {
		return nil
	}

	return entry
}

func (s *statePersister) persistEventIndexFile() error {
	indexData, err := json.Marshal(s.eventIndex)
	if err != nil {
		return err
	}

	indexFilePath := eventIndexFilePath(s.stateDir)
	return afero.WriteFile(s.fs, indexFilePath, indexData, 0644)
}

func instanceChunkFilePath(stateDir string, chunkIndex int) string {
	return path.Join(
		stateDir,
		fmt.Sprintf("instances_c%d.json", chunkIndex),
	)
}

func instanceIndexFilePath(stateDir string) string {
	return path.Join(stateDir, "instance_index.json")
}

func eventIndexFilePath(stateDir string) string {
	return path.Join(stateDir, "event_index.json")
}

func resourceDriftChunkFilePath(stateDir string, chunkIndex int) string {
	return path.Join(
		stateDir,
		fmt.Sprintf("resource_drift_c%d.json", chunkIndex),
	)
}

func resourceDriftIndexFilePath(stateDir string) string {
	return path.Join(stateDir, "resource_drift_index.json")
}

func eventPartitionFilePath(stateDir string, partitionName string) string {
	return path.Join(
		stateDir,
		fmt.Sprintf("%s.json", toPartitionFileBaseName(partitionName)),
	)
}

func toPartitionFileBaseName(partitionName string) string {
	return fmt.Sprintf("events__%s", partitionName)
}

func toPersistedInstanceState(instance *state.InstanceState) *persistedInstanceState {
	childBlueprintsMap := map[string]string{}
	for childName, childBlueprint := range instance.ChildBlueprints {
		childBlueprintsMap[childName] = childBlueprint.InstanceID
	}

	return &persistedInstanceState{
		InstanceID:                 instance.InstanceID,
		InstanceName:               instance.InstanceName,
		Status:                     instance.Status,
		LastStatusUpdateTimestamp:  instance.LastStatusUpdateTimestamp,
		LastDeployedTimestamp:      instance.LastDeployedTimestamp,
		LastDeployAttemptTimestamp: instance.LastDeployAttemptTimestamp,
		ResourceIDs:                instance.ResourceIDs,
		Resources:                  instance.Resources,
		Links:                      instance.Links,
		Metadata:                   instance.Metadata,
		Exports:                    instance.Exports,
		ChildDependencies:          instance.ChildDependencies,
		ChildBlueprints:            childBlueprintsMap,
		Durations:                  instance.Durations,
	}
}
