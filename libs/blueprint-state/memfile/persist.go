package memfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"sync"

	"github.com/spf13/afero"
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

func instanceChunkFilePath(stateDir string, chunkIndex int) string {
	return path.Join(
		stateDir,
		fmt.Sprintf("instances_c%d.json", chunkIndex),
	)
}

func instanceIndexFilePath(stateDir string) string {
	return path.Join(stateDir, "instance_index.json")
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
