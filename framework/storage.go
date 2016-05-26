package framework

import (
	"fmt"

	"strings"
	"sync"
	"time"

	"github.com/gocql/gocql"
	yaml "gopkg.in/yaml.v2"
)

type Runner interface {
	Run(*RunRequest, *RunContext) error
	GetStack() *Stack
}

type Storage interface {
	GetAll() ([]*Stack, error)
	GetStack(string) (*Stack, error)
	GetStackRunner(string) (Runner, error)
	StoreStack(*Stack) error
	RemoveStack(string, bool) error
	GetLayersStack(string) (Merger, error)

	SaveStackStatus(name string, zone string, status StackStatus) error
	SaveApplicationStatus(stack string, zone string, applicationName string, status ApplicationStatus) error
	SaveStackVariables(stack string, zone string, variables *Variables) error
	GetStackState(name string, zone string) (*StackState, error)
	GetState() (*StackDeployState, error)
}

type CassandraStorage struct {
	connection *gocql.Session
	keyspace   string
	lock       sync.Mutex
}

func NewCassandraStorageRetryBackoff(cluster []string, keyspace string, retries int, backoff time.Duration, proto int) (Storage, *gocql.Session, error) {
	var err error
	var storage Storage
	var connection *gocql.Session
	for i := 0; i < retries; i++ {
		Logger.Info("Trying to connect to cassandra cluster at %s: %d try", strings.Join(cluster, ","), i+1)
		storage, connection, err = NewCassandraStorage(cluster, keyspace, proto)
		if err == nil {
			return storage, connection, nil
		}
		Logger.Debug("Error: %s", err)
		time.Sleep(backoff)
	}
	return nil, nil, err
}

func NewCassandraStorage(addresses []string, keyspace string, proto int) (Storage, *gocql.Session, error) {
	cluster := gocql.NewCluster(addresses...)
	cluster.ProtoVersion = proto
	cluster.Timeout = 5 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, nil, err
	}

	storage := &CassandraStorage{connection: session, keyspace: keyspace}
	return storage, session, storage.init()
}

func (cs *CassandraStorage) GetAll() ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	query := fmt.Sprintf(`SELECT name, parent, applications from %s.stacks`, cs.keyspace)

	var name string
	var parent string
	var apps string
	iter := cs.connection.Query(query).Iter()
	for iter.Scan(&name, &parent, &apps) {
		stack := new(Stack)
		stack.Name = name
		stack.From = parent

		err := yaml.Unmarshal([]byte(apps), &stack.Applications)
		if err != nil {
			Logger.Info("Unable get stack %s from Cassandra: %s", name, err)
			return nil, fmt.Errorf("Unable get stack %s from Cassandra: %s", name, err)
		}

		stacks = append(stacks, stack)
	}

	return stacks, nil
}

func (cs *CassandraStorage) GetStackRunner(name string) (Runner, error) {
	return cs.GetStack(name)
}

func (cs *CassandraStorage) GetStack(name string) (*Stack, error) {
	stack := &Stack{}
	query := fmt.Sprintf(`SELECT name, parent, applications from %s.stacks where name = ? LIMIT 1`, cs.keyspace)
	var parent string
	var apps string
	err := cs.connection.Query(query, name).Scan(&stack.Name, &parent, &apps)
	if err != nil {
		Logger.Info("Unable get stack %s from Cassandra: %s", name, err)
		if err == gocql.ErrNotFound {
			return nil, ErrStackDoesNotExist
		} else {
			return nil, err
		}
	}

	err = yaml.Unmarshal([]byte(apps), &stack.Applications)
	if err != nil {
		Logger.Info("Unable get stack %s from Cassandra: %s", name, err)
		return nil, fmt.Errorf("Unable get stack %s from Cassandra: %s", name, err)
	}

	if parent != "" {
		parentStack, err := cs.GetStack(parent)
		if err != nil {
			return nil, err
		}

		parentStack.Merge(stack)
		return parentStack, nil
	}

	return stack, nil
}

func (cs *CassandraStorage) GetLayersStack(name string) (Merger, error) {
	zone, err := cs.GetLayer(name)
	if err != nil {
		return nil, err
	}
	if zone.Stack.From == "" {
		return zone.Stack, nil
	}
	cluster, err := cs.GetLayer(zone.Stack.From)
	if err != nil {
		return nil, err
	}
	if cluster.Stack.From == "" {
		return cluster.Stack, nil
	}
	datacenter, err := cs.GetLayer(cluster.Stack.From)
	if err != nil {
		return nil, err
	}
	err = datacenter.Merge(cluster)
	if err != nil {
		return nil, err
	}
	err = datacenter.Merge(zone)
	if err != nil {
		return nil, err
	}
	return datacenter.Stack, nil
}

func (cs *CassandraStorage) GetLayer(name string) (*Layer, error) {
	stack, err := cs.GetStack(name)
	if err != nil {
		return nil, err
	}
	layer := NewLayer(stack)
	if layer != nil {
		return layer, nil
	}
	return nil, fmt.Errorf("Can't create layer %s with level %d", name, stack.Layer)
}

func (cs *CassandraStorage) StoreStack(stack *Stack) error {
	cs.lock.Lock()
	defer cs.lock.Unlock()

	exists, err := cs.exists(stack.Name)
	if err != nil {
		return err
	}

	if exists {
		Logger.Info("Stack %s already exists", stack.Name)
		return ErrStackExists
	}

	if stack.From != "" {
		exists, err = cs.exists(stack.From)
		if !exists {
			Logger.Info("Parent stack %s does not exist", stack.From)
			return ErrStackDoesNotExist
		}
	}

	apps, err := yaml.Marshal(stack.Applications)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`INSERT INTO %s.stacks (name, parent, applications) VALUES (?, ?, ?)`, cs.keyspace)
	err = cs.connection.Query(query, stack.Name, stack.From, apps).Exec()
	if err != nil {
		return err
	}
	return nil
}

func (cs *CassandraStorage) RemoveStack(stack string, force bool) error {
	cs.lock.Lock()
	defer cs.lock.Unlock()

	exists, err := cs.exists(stack)
	if err != nil {
		return err
	}

	if !exists {
		Logger.Info("Stack %s does not exist", stack)
		return ErrStackDoesNotExist
	}

	return cs.remove(stack, force)
}

func (cs *CassandraStorage) remove(stack string, force bool) error {
	Logger.Info("Removing %s with force = %t", stack, force)
	childrenQuery := fmt.Sprintf("SELECT name FROM %s.stacks WHERE parent = ?", cs.keyspace)

	children := make([]string, 0)
	name := ""
	iter := cs.connection.Query(childrenQuery, stack).Iter()
	for iter.Scan(&name) {
		if force {
			err := cs.remove(name, force)
			if err != nil {
				return err
			}
		} else {
			children = append(children, name)
		}
	}

	Logger.Debug("%s children: %s", stack, children)
	if len(children) > 0 {
		return fmt.Errorf("There are stacks depending on %s. Either remove them first or force deletion. Dependant stacks:\n%s",
			stack, strings.Join(children, "\n"))
	}

	query := fmt.Sprintf("DELETE FROM %s.stacks WHERE name = ?", cs.keyspace)
	return cs.connection.Query(query, stack).Exec()
}

func (cs *CassandraStorage) init() error {
	query := fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s WITH replication = { 'class' : 'SimpleStrategy', 'replication_factor' : 1 }", cs.keyspace)
	err := cs.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.stacks (name text, parent text, applications text, PRIMARY KEY(name))", cs.keyspace)
	err = cs.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = fmt.Sprintf("CREATE INDEX IF NOT EXISTS stacks_parent_idx ON %s.stacks (parent)", cs.keyspace)
	err = cs.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = cs.prepareQuery("CREATE TABLE IF NOT EXISTS %s.stack_states (name text, zone text, state int, time timestamp, PRIMARY KEY(name, zone))")
	err = cs.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = cs.prepareQuery("CREATE TABLE IF NOT EXISTS %s.application_states (stack text, zone text, name text, state int, PRIMARY KEY(stack, zone, name))")
	err = cs.connection.Query(query).Exec()
	if err != nil {
		return err
	}

	query = cs.prepareQuery("CREATE TABLE IF NOT EXISTS %s.stack_variables (stack text, zone text, key text, value text, variable_type text, PRIMARY KEY(stack, zone, key))")
	return cs.connection.Query(query).Exec()
}

func (cs *CassandraStorage) exists(name string) (bool, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) from %s.stacks where name = ?`, cs.keyspace)
	var count int
	err := cs.connection.Query(query, name).Scan(&count)
	if err != nil {
		Logger.Info("Unable get stack %s from Cassandra: %s", name, err)
		return false, err
	}

	return count > 0, nil
}

func (css *CassandraStorage) SaveStackStatus(name string, zone string, status StackStatus) error {
	query := css.prepareQuery("INSERT INTO %s.stack_states (name, zone, state, time) VALUES (?, ?, ?, ?)")
	return css.connection.Query(query, name, zone, status, time.Now()).Exec()
}

func (css *CassandraStorage) SaveApplicationStatus(stack string, zone string, applicationName string, status ApplicationStatus) error {
	query := css.prepareQuery("INSERT INTO %s.application_states (stack, zone, name, state) VALUES (?, ?, ?, ?)")
	return css.connection.Query(query, stack, zone, applicationName, status).Exec()
}

func (css *CassandraStorage) SaveStackVariables(stack string, zone string, variables *Variables) error {
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

func (css *CassandraStorage) saveStackVariable(stack string, zone string, varType string, key string, value string) error {
	query := css.prepareQuery("INSERT INTO %s.stack_variables (stack, zone, key, value, variable_type) VALUES (?, ?, ?, ?, ?)")
	return css.connection.Query(query, stack, zone, key, value, varType).Exec()
}

func (css *CassandraStorage) GetStackState(name string, zone string) (*StackState, error) {
	var status int
	var timestamp time.Time
	err := css.connection.Query(css.prepareQuery("SELECT state, time FROM %s.stack_states WHERE name = ? AND zone = ?"), name, zone).Scan(&status, &timestamp)
	if err != nil {
		if err == gocql.ErrNotFound {
			return nil, ErrStackStateDoesNotExist
		} else {
			return nil, err
		}
	}

	state := newStackState(name, zone)
	state.Status = StackStatus(status)
	state.Timestamp = timestamp
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

func (css *CassandraStorage) GetState() (*StackDeployState, error) {
	state := NewStackDeployState()

	var name string
	var zone string
	iter := css.connection.Query(css.prepareQuery("SELECT name, zone FROM %s.stack_states")).Iter()
	for iter.Scan(&name, &zone) {
		_, err := iter.RowData()
		if err != nil {
			return nil, err
		}

		stackState, err := css.GetStackState(name, zone)
		if err != nil {
			return nil, err
		}

		state.RunningStacks = append(state.RunningStacks, stackState)
	}

	stacks, err := css.GetAll()
	if err != nil {
		return nil, err
	}

	for _, stack := range stacks {
		state.Stacks = append(state.Stacks, stack.String())
	}

	return state, nil
}

func (css *CassandraStorage) prepareQuery(query string) string {
	return fmt.Sprintf(query, css.keyspace)
}

type InMemoryStorage struct {
	stacks      map[string]*Stack
	stackStates map[string]map[string]*StackState
	lock        sync.Mutex
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		stacks:      make(map[string]*Stack),
		stackStates: make(map[string]map[string]*StackState),
	}
}

func (s *InMemoryStorage) GetAll() ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	for _, stack := range s.stacks {
		stacks = append(stacks, stack)
	}

	return stacks, nil
}

func (s *InMemoryStorage) GetStack(name string) (*Stack, error) {
	stack, exists := s.stacks[name]
	if !exists {
		return nil, ErrStackDoesNotExist
	}

	return stack, nil
}

func (s *InMemoryStorage) GetStackRunner(name string) (Runner, error) {
	return s.GetStack(name)
}

func (s *InMemoryStorage) StoreStack(stack *Stack) error {
	_, exists := s.stacks[stack.Name]
	if exists {
		return ErrStackExists
	}

	s.stacks[stack.Name] = stack
	return nil
}

func (s *InMemoryStorage) RemoveStack(name string, force bool) error {
	_, exists := s.stacks[name]
	if !exists {
		return ErrStackDoesNotExist
	}

	return s.remove(name, force)
}

func (s *InMemoryStorage) GetLayersStack(name string) (Merger, error) {
	zone, err := s.GetLayer(name)
	if err != nil {
		return nil, err
	}
	if zone.Stack.From == "" {
		return zone.Stack, nil
	}
	cluster, err := s.GetLayer(zone.Stack.From)
	if err != nil {
		return nil, err
	}
	if cluster.Stack.From == "" {
		return cluster.Stack, nil
	}
	datacenter, err := s.GetLayer(cluster.Stack.From)
	if err != nil {
		return nil, err
	}
	err = datacenter.Merge(cluster)
	if err != nil {
		return nil, err
	}
	err = datacenter.Merge(zone)
	if err != nil {
		return nil, err
	}
	return datacenter.Stack, nil
}

func (s *InMemoryStorage) GetLayer(name string) (*Layer, error) {
	stack, err := s.GetStack(name)
	if err != nil {
		return nil, err
	}
	layer := NewLayer(stack)
	if layer != nil {
		return layer, nil
	}
	return nil, fmt.Errorf("Can't create layer %s with level %d", name, stack.Layer)
}

func (s *InMemoryStorage) remove(name string, force bool) error {
	Logger.Info("Removing %s with force = %t", name, force)

	children := make([]string, 0)
	for _, stack := range s.stacks {
		if stack.From == name {
			if force {
				err := s.remove(stack.Name, force)
				if err != nil {
					return err
				}
			} else {
				children = append(children, stack.Name)
			}
		}
	}

	Logger.Debug("%s children: %s", name, children)
	if len(children) > 0 {
		return fmt.Errorf("There are stacks depending on %s. Either remove them first or force deletion. Dependant stacks:\n%s",
			name, strings.Join(children, "\n"))
	}

	delete(s.stacks, name)
	return nil
}

func (s *InMemoryStorage) SaveStackStatus(name string, zone string, status StackStatus) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	_, exists := s.stackStates[name]
	if !exists {
		s.stackStates[name] = make(map[string]*StackState)
	}

	if s.stackStates[name][zone] == nil {
		s.stackStates[name][zone] = newStackState(name, zone)
	}
	s.stackStates[name][zone].Status = status
	s.stackStates[name][zone].Timestamp = time.Now()
	return nil
}

func (s *InMemoryStorage) SaveApplicationStatus(stack string, zone string, applicationName string, status ApplicationStatus) error {
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

func (s *InMemoryStorage) SaveStackVariables(stack string, zone string, variables *Variables) error {
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

func (s *InMemoryStorage) GetStackState(stack string, zone string) (*StackState, error) {
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

func (s *InMemoryStorage) GetState() (*StackDeployState, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	state := NewStackDeployState()
	for _, zoneStates := range s.stackStates {
		for _, stackState := range zoneStates {
			state.RunningStacks = append(state.RunningStacks, stackState)
		}
	}

	for _, stack := range s.stacks {
		state.Stacks = append(state.Stacks, s.stacks[stack.Name].String())
	}

	return state, nil
}

type StackState struct {
	Timestamp    time.Time
	Name         string
	Zone         string
	Status       StackStatus
	Variables    *Variables
	Applications map[string]ApplicationStatus
}

func newStackState(name string, zone string) *StackState {
	return &StackState{
		Name:         name,
		Zone:         zone,
		Applications: make(map[string]ApplicationStatus),
	}
}

type StackStateSlice []*StackState

func (s StackStateSlice) Len() int           { return len(s) }
func (s StackStateSlice) Less(i, j int) bool { return s[i].Timestamp.Before(s[j].Timestamp) }
func (s StackStateSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type StackDeployState struct {
	//TODO add layers info too
	Stacks        []string
	RunningStacks []*StackState
}

func NewStackDeployState() *StackDeployState {
	return &StackDeployState{
		Stacks:        make([]string, 0),
		RunningStacks: make([]*StackState, 0),
	}
}

func (s *StackDeployState) GetStacks() []*Stack {
	stacks := make([]*Stack, len(s.Stacks))
	for i, rawStack := range s.Stacks {
		stack, err := UnmarshalStack([]byte(rawStack))
		if err != nil {
			panic(err)
		}

		stacks[i] = stack
	}

	return stacks
}
