package state

import (
	"agentgo/logger"
	"encoding/json"
	"os"
	"sync"
)

// State is state.json
type State struct {
	data stateData

	l    sync.Mutex
	path string
}

type stateData struct {
	AgentUUID     string `json:"agent_uuid"`
	AgentPassword string `json:"password"`

	Cache map[string]json.RawMessage
}

// Load load state.json file
func Load(path string) (*State, error) {
	state := State{
		path: path,
	}
	f, err := os.Open(path)
	if err != nil && os.IsNotExist(err) {
		return &state, nil
	} else if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&state.data)
	if state.data.Cache == nil {
		state.data.Cache = make(map[string]json.RawMessage)
	}
	return &state, err
}

// Save will write back the State to state.json
func (s *State) Save() error {
	s.l.Lock()
	defer s.l.Unlock()
	return s.save()
}

func (s *State) save() error {
	err := s.saveTo(s.path + ".tmp")
	if err != nil {
		return err
	}
	err = os.Rename(s.path+".tmp", s.path)
	return err
}

func (s *State) saveTo(path string) error {
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer w.Close()
	encoder := json.NewEncoder(w)
	err = encoder.Encode(s.data)
	if err != nil {
		return err
	}
	_ = w.Sync()
	return nil
}

// AgentID returns the agent UUID for Bleemeo
func (s *State) AgentID() string {
	s.l.Lock()
	defer s.l.Unlock()
	return s.data.AgentUUID
}

// AgentPassword returns the agent password for Bleemeo
func (s *State) AgentPassword() string {
	s.l.Lock()
	defer s.l.Unlock()
	return s.data.AgentPassword
}

// SetAgentIDPassword save the agent UUID and password for Bleemeo
func (s *State) SetAgentIDPassword(agentID string, password string) {
	s.l.Lock()
	defer s.l.Unlock()

	s.data.AgentUUID = agentID
	s.data.AgentPassword = password
	err := s.save()
	if err != nil {
		logger.Printf("Unable to save state.json: %v", err)
	}
}

// SetCache save a cache object
func (s *State) SetCache(key string, object interface{}) error {
	s.l.Lock()
	defer s.l.Unlock()

	buffer, err := json.Marshal(object)
	if err != nil {
		return err
	}
	s.data.Cache[key] = json.RawMessage(buffer)
	err = s.save()
	if err != nil {
		logger.Printf("Unable to save state.json: %v", err)
	}
	return nil
}

// Cache get a cache object
func (s *State) Cache(key string, result interface{}) error {
	s.l.Lock()
	defer s.l.Unlock()

	buffer, ok := s.data.Cache[key]
	if !ok {
		return nil
	}
	err := json.Unmarshal(buffer, &result)
	return err
}