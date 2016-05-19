package framework

import (
	"fmt"
	"github.com/gocql/gocql"
	"sync"
)

type StateStorage interface {
	SaveStackStatus(name string, zone string, status StackStatus) error
	SaveApplicationStatus(stack string, zone string, applicationName string, status ApplicationStatus) error
	SaveStackVariables(stack string, zone string, variables *Variables) error
	GetStackState(name string, zone string) (*StackState, error)
}

type CassandraStateStorage struct {
	connection *gocql.Session
	keyspace   string
}

func NewCassandraStateStorage(connection *gocql.Session, keyspace string) (*CassandraStateStorage, error) {
	storage := &CassandraStateStorage{connection: connection, keyspace: keyspace}
	return storage, storage.init()
}

func (css *CassandraStateStorage) SaveStackStatus(name string, zone string, status StackStatus) error {
	query := css.prepareQuery("INSERT INTO %s.stack_states (name, zone, state) VALUES (?, ?, ?)")
	return css.connection.Query(query, name, zone, status).Exec()
}

func (css *CassandraStateStorage) SaveApplicationStatus(stack string, zone string, applicationName string, status ApplicationStatus) error {
	query := css.prepareQuery("INSERT INTO %s.application_states (stack, zone, name, state) VALUES (?, ?, ?, ?)")
	return css.connection.Query(query, stack, zone, applicationName, status).Exec()
}

func (css *CassandraStateStorage) SaveStackVariables(stack string, zone string, variables *Variables) error {
	for k, v := range variables.globalVariables {
		err := css.saveStackVariable(stack, zone, "global", k, v)
		if err != nil {
			return err
		}
	}

	for k, v := range variables.arbitraryVariables {
		err := css.saveStackVariable(stack, zone, "arbitrary", k, v)
		if err != nil {
			return err
		}
	}

	for k, v := range variables.stackVariables {
		err := css.saveStackVariable(stack, zone, "stack", k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (css *CassandraStateStorage) saveStackVariable(stack string, zone string, varType string, key string, value string) error {
	query := css.prepareQuery("INSERT INTO %s.stack_variables (stack, zone, key, value, variable_type) VALUES (?, ?, ?, ?, ?)")
	return css.connection.Query(query, stack, zone, key, value, varType).Exec()
}

func (css *CassandraStateStorage) GetStackState(name string, zone string) (*StackState, error) {
	var status int
	err := css.connection.Query(css.prepareQuery("SELECT state FROM %s.stack_states WHERE name = ? AND zone = ?"), name, zone).Scan(&status)
	if err != nil {
		return nil, err
	}

	state := newStackState(name, zone, StackStatus(status))
	iter := css.connection.Query(css.prepareQuery("SELECT name, state FROM %s.application_states WHERE stack = ? AND zone = ?"), name, zone).Iter()
	state.Applications = make(map[string]ApplicationStatus)
	var appName string
	var appStatus int
	for iter.Scan(&appName, &appStatus) {
		_, err = iter.RowData()
		if err != nil {
			return nil, err
		}
		state.Applications[appName] = ApplicationStatus(appStatus)
	}

	iter = css.connection.Query(css.prepareQuery("SELECT key, value, variable_type FROM %s.stack_variables WHERE stack = ? AND zone = ?"), name, zone).Iter()
	state.Variables = NewVariables()
	var varType string
	var varKey string
	var varValue string
	for iter.Scan(&varKey, &varValue, &varType) {
		_, err = iter.RowData()
		if err != nil {
			return nil, err
		}

		switch varType {
		case "global":
			state.Variables.SetGlobalVariable(varKey, varValue)
		case "arbitrary":
			state.Variables.SetArbitraryVariable(varKey, varValue)
		case "stack":
			state.Variables.SetStackVariable(varKey, varValue)
		default:
			panic(fmt.Sprintf("Unknown stored variable type %s", varType))
		}
	}

	return state, nil
}

func (css *CassandraStateStorage) init() error {
	query := css.prepareQuery("CREATE TABLE IF NOT EXISTS %s.stack_states (name text, zone text, state int, PRIMARY KEY(name, zone))")
	err := css.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = css.prepareQuery("CREATE TABLE IF NOT EXISTS %s.application_states (stack text, zone text, name text, state int, PRIMARY KEY(stack, zone, name))")
	err = css.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = css.prepareQuery("CREATE TABLE IF NOT EXISTS %s.stack_variables (stack text, zone text, key text, value text, variable_type text, PRIMARY KEY(stack, zone, key))")
	return css.connection.Query(query).Exec()
}

func (css *CassandraStateStorage) prepareQuery(query string) string {
	return fmt.Sprintf(query, css.keyspace)
}

type InMemoryStateStorage struct {
	stackStates map[string]map[string]*StackState
	lock        sync.Mutex
}

func NewInMemoryStateStorage() *InMemoryStateStorage {
	return &InMemoryStateStorage{
		stackStates: make(map[string]map[string]*StackState),
	}
}

func (s *InMemoryStateStorage) SaveStackStatus(name string, zone string, status StackStatus) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	nameStates, exists := s.stackStates[name]
	if !exists {
		s.stackStates[name] = make(map[string]*StackState)
		nameStates = s.stackStates[name]
	}

	_, exists = nameStates[zone]
	if exists {
		return ErrStackStateExists
	}

	s.stackStates[name][zone] = newStackState(name, zone, status)
	return nil
}

func (s *InMemoryStateStorage) SaveApplicationStatus(stack string, zone string, applicationName string, status ApplicationStatus) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	nameStates, exists := s.stackStates[stack]
	if !exists {
		return ErrStackStateDoesNotExist
	}

	stackState, exists := nameStates[zone]
	if !exists {
		return ErrStackStateDoesNotExist
	}

	stackState.Applications[applicationName] = status
	return nil
}

func (s *InMemoryStateStorage) SaveStackVariables(stack string, zone string, variables *Variables) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	nameStates, exists := s.stackStates[stack]
	if !exists {
		return ErrStackStateDoesNotExist
	}

	stackState, exists := nameStates[zone]
	if !exists {
		return ErrStackStateDoesNotExist
	}

	stackState.Variables = variables
	return nil
}

func (s *InMemoryStateStorage) GetStackState(stack string, zone string) (*StackState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	nameStates, exists := s.stackStates[stack]
	if !exists {
		return nil, ErrStackStateDoesNotExist
	}

	stackState, exists := nameStates[zone]
	if !exists {
		return nil, ErrStackStateDoesNotExist
	}

	return stackState, nil
}

type StackState struct {
	Name         string
	Zone         string
	Status       StackStatus
	Variables    *Variables
	Applications map[string]ApplicationStatus
}

func newStackState(name string, zone string, status StackStatus) *StackState {
	return &StackState{
		Name:         name,
		Zone:         zone,
		Status:       status,
		Applications: make(map[string]ApplicationStatus),
	}
}
