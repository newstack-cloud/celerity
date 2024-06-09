package languageserver

import "sync"

type State struct {
	hasWorkspaceFolderCapability bool
	hasConfigurationCapability   bool
	documentSettings             map[string]*DocSettings
	lock                         sync.Mutex
}

func NewState() *State {
	return &State{
		documentSettings: make(map[string]*DocSettings),
	}
}

type DocSettings struct {
	Trace               DocTraceSettings `json:"trace"`
	MaxNumberOfProblems int              `json:"maxNumberOfProblems"`
}

type DocTraceSettings struct {
	Server string `json:"server"`
}

func (s *State) HasWorkspaceFolderCapability() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.hasWorkspaceFolderCapability
}

func (s *State) SetWorkspaceFolderCapability(value bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.hasWorkspaceFolderCapability = value
}

func (s *State) HasConfigurationCapability() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.hasConfigurationCapability
}

func (s *State) SetConfigurationCapability(value bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.hasConfigurationCapability = value
}

func (s *State) ClearDocSettings() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentSettings = make(map[string]*DocSettings)
}
