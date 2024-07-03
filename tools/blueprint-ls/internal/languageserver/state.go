package languageserver

import (
	"sync"

	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

type State struct {
	hasWorkspaceFolderCapability bool
	hasConfigurationCapability   bool
	documentSettings             map[string]*DocSettings
	documentContent              map[string]string
	positionEncodingKind         lsp.PositionEncodingKind
	lock                         sync.Mutex
}

func NewState() *State {
	return &State{
		documentSettings: make(map[string]*DocSettings),
		documentContent:  make(map[string]string),
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

func (s *State) SetPositionEncodingKind(value lsp.PositionEncodingKind) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.positionEncodingKind = value
}

func (s *State) GetPositionEncodingKind() lsp.PositionEncodingKind {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.positionEncodingKind
}

func (s *State) GetDocumentContent(uri string) *string {
	s.lock.Lock()
	defer s.lock.Unlock()
	content, ok := s.documentContent[uri]
	if !ok {
		return nil
	}
	return &content
}

func (s *State) SetDocumentContent(uri string, content string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentContent[uri] = content
}

func (s *State) ClearDocSettings() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentSettings = make(map[string]*DocSettings)
}
