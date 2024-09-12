package languageserver

import (
	"sync"

	"github.com/two-hundred/celerity/libs/blueprint/schema"
	lsp "github.com/two-hundred/ls-builder/lsp_3_17"
)

type State struct {
	hasWorkspaceFolderCapability bool
	hasConfigurationCapability   bool
	documentSettings             map[string]*DocSettings
	documentContent              map[string]string
	documentSchemas              map[string]*schema.Blueprint
	documentPositionMaps         map[string]map[string][]*schema.TreeNode
	documentTrees                map[string]*schema.TreeNode
	positionEncodingKind         lsp.PositionEncodingKind
	lock                         sync.Mutex
}

func NewState() *State {
	return &State{
		documentSettings:     make(map[string]*DocSettings),
		documentContent:      make(map[string]string),
		documentSchemas:      make(map[string]*schema.Blueprint),
		documentPositionMaps: make(map[string]map[string][]*schema.TreeNode),
		documentTrees:        make(map[string]*schema.TreeNode),
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

func (s *State) GetDocumentSchema(uri string) *schema.Blueprint {
	s.lock.Lock()
	defer s.lock.Unlock()
	blueprint, ok := s.documentSchemas[uri]
	if !ok {
		return nil
	}
	return blueprint
}

func (s *State) SetDocumentSchema(uri string, blueprint *schema.Blueprint) {
	if blueprint == nil {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentSchemas[uri] = blueprint
}

func (s *State) GetDocumentPositionMapNodes(uri string, positionKey string) []*schema.TreeNode {
	s.lock.Lock()
	defer s.lock.Unlock()
	positionMap, ok := s.documentPositionMaps[uri]
	if !ok {
		return nil
	}
	nodes, ok := positionMap[positionKey]
	if !ok {
		return nil
	}

	return nodes
}

func (s *State) GetDocumentPositionMapSmallestNode(uri string, positionKey string) *schema.TreeNode {
	s.lock.Lock()
	defer s.lock.Unlock()
	positionMap, ok := s.documentPositionMaps[uri]
	if !ok {
		return nil
	}
	nodes, ok := positionMap[positionKey]
	if !ok {
		return nil
	}

	// The last element in the list is expected to be the smallest node
	// assuming the nodes are traversed bottom up in producing the
	// position map.
	return nodes[len(nodes)-1]
}

func (s *State) SetDocumentTree(uri string, tree *schema.TreeNode) {
	if tree == nil {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentTrees[uri] = tree
}

func (s *State) GetDocumentTree(uri string) *schema.TreeNode {
	s.lock.Lock()
	defer s.lock.Unlock()
	tree, ok := s.documentTrees[uri]
	if !ok {
		return nil
	}

	return tree
}

func (s *State) SetDocumentPositionMap(uri string, posMap map[string][]*schema.TreeNode) {
	if posMap == nil {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentPositionMaps[uri] = posMap
}

func (s *State) ClearDocSettings() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.documentSettings = make(map[string]*DocSettings)
}
