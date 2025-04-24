package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/two-hundred/celerity/libs/blueprint-state/manage"
	"github.com/two-hundred/celerity/libs/blueprint/state"
)

type SaveBlueprintFixture struct {
	InstanceState *state.InstanceState
	Update        bool
}

func SetupSaveBlueprintFixtures(dirPath string, updates []int) (map[int]SaveBlueprintFixture, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	saveBlueprintFixtures := make(map[int]SaveBlueprintFixture)
	for i := 1; i <= len(dirEntries); i++ {
		isUpdate := slices.Contains(updates, i)
		fixture, err := loadSaveBlueprintFixture(i, dirPath, isUpdate)
		if err != nil {
			return nil, err
		}
		saveBlueprintFixtures[i] = fixture
	}

	return saveBlueprintFixtures, nil
}

func loadSaveBlueprintFixture(fixtureNumber int, dirPath string, isUpdate bool) (SaveBlueprintFixture, error) {
	fileName := fixtureFileName(fixtureNumber)
	filePath := path.Join(dirPath, fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SaveBlueprintFixture{}, err
	}

	instanceState := &state.InstanceState{}
	err = json.Unmarshal(data, instanceState)
	if err != nil {
		return SaveBlueprintFixture{}, err
	}

	return SaveBlueprintFixture{
		InstanceState: instanceState,
		Update:        isUpdate,
	}, nil
}

type SaveResourceFixture struct {
	ResourceState *state.ResourceState
	Update        bool
}

func SetupSaveResourceFixtures(dirPath string, updates []int) (map[int]SaveResourceFixture, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	saveResourceFixtures := make(map[int]SaveResourceFixture)
	for i := 1; i <= len(dirEntries); i++ {
		isUpdate := slices.Contains(updates, i)
		fixture, err := loadSaveResourceFixture(i, isUpdate)
		if err != nil {
			return nil, err
		}
		saveResourceFixtures[i] = fixture
	}

	return saveResourceFixtures, nil
}

func loadSaveResourceFixture(fixtureNumber int, isUpdate bool) (SaveResourceFixture, error) {
	fileName := fixtureFileName(fixtureNumber)
	filePath := path.Join(saveInputDir(), "resources", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SaveResourceFixture{}, err
	}

	resourceState := &state.ResourceState{}
	err = json.Unmarshal(data, resourceState)
	if err != nil {
		return SaveResourceFixture{}, err
	}

	return SaveResourceFixture{
		ResourceState: resourceState,
		Update:        isUpdate,
	}, nil
}

type SaveResourceDriftFixture struct {
	DriftState *state.ResourceDriftState
	Update     bool
}

func SetupSaveResourceDriftFixtures(dirPath string, updates []int) (map[int]SaveResourceDriftFixture, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	saveResourceDriftFixtures := make(map[int]SaveResourceDriftFixture)
	for i := 1; i <= len(dirEntries); i++ {
		isUpdate := slices.Contains(updates, i)
		fixture, err := loadSaveResourceDriftFixture(i, isUpdate)
		if err != nil {
			return nil, err
		}
		saveResourceDriftFixtures[i] = fixture
	}

	return saveResourceDriftFixtures, nil
}

func loadSaveResourceDriftFixture(fixtureNumber int, isUpdate bool) (SaveResourceDriftFixture, error) {
	fileName := fixtureFileName(fixtureNumber)
	filePath := path.Join(saveInputDir(), "resource-drift", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SaveResourceDriftFixture{}, err
	}

	driftState := &state.ResourceDriftState{}
	err = json.Unmarshal(data, driftState)
	if err != nil {
		return SaveResourceDriftFixture{}, err
	}

	return SaveResourceDriftFixture{
		DriftState: driftState,
		Update:     isUpdate,
	}, nil
}

type SaveLinkFixture struct {
	LinkState *state.LinkState
	Update    bool
}

func SetupSaveLinkFixtures(dirPath string, updates []int) (map[int]SaveLinkFixture, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	saveLinkFixtures := make(map[int]SaveLinkFixture)
	for i := 1; i <= len(dirEntries); i++ {
		isUpdate := slices.Contains(updates, i)
		fixture, err := loadSaveLinkFixture(i, isUpdate)
		if err != nil {
			return nil, err
		}
		saveLinkFixtures[i] = fixture
	}

	return saveLinkFixtures, nil
}

func loadSaveLinkFixture(fixtureNumber int, isUpdate bool) (SaveLinkFixture, error) {
	fileName := fmt.Sprintf("%d.json", fixtureNumber)
	filePath := path.Join(saveInputDir(), "links", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SaveLinkFixture{}, err
	}

	linkState := &state.LinkState{}
	err = json.Unmarshal(data, linkState)
	if err != nil {
		return SaveLinkFixture{}, err
	}

	return SaveLinkFixture{
		LinkState: linkState,
		Update:    isUpdate,
	}, nil
}

type SaveEventFixture struct {
	Event *manage.Event
}

func SetupSaveEventFixtures(dirPath string) (map[int]SaveEventFixture, error) {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	saveEventFixtures := make(map[int]SaveEventFixture)
	for i := 1; i <= len(dirEntries); i++ {
		fixture, err := loadSaveEventFixture(i)
		if err != nil {
			return nil, err
		}
		saveEventFixtures[i] = fixture
	}

	return saveEventFixtures, nil
}

func loadSaveEventFixture(fixtureNumber int) (SaveEventFixture, error) {
	fileName := fmt.Sprintf("%d.json", fixtureNumber)
	filePath := path.Join(saveInputDir(), "events", fileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SaveEventFixture{}, err
	}

	event := &manage.Event{}
	err = json.Unmarshal(data, event)
	if err != nil {
		return SaveEventFixture{}, err
	}

	return SaveEventFixture{
		Event: event,
	}, nil
}

func saveInputDir() string {
	return path.Join("__testdata", "save-input")
}

func fixtureFileName(fixtureNumber int) string {
	return fmt.Sprintf("%d.json", fixtureNumber)
}
