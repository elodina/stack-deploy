package framework

import (
	"fmt"

	"strings"
	"sync"
	"time"

	"github.com/gocql/gocql"
	yaml "gopkg.in/yaml.v2"
)

type Storage interface {
	GetAll() ([]*Stack, error)
	GetStack(string) (*Stack, error)
	StoreStack(*Stack) error
	RemoveStack(string, bool) error
	Init() error
}

type CassandraStorage struct {
	connection *gocql.Session
	keyspace   string
	lock       sync.Mutex
}

func NewCassandraStorageRetryBackoff(cluster []string, keyspace string, retries int, backoff time.Duration) (Storage, error) {
	var err error
	var storage Storage
	for i := 0; i < retries; i++ {
		Logger.Info("Trying to connect to cassandra cluster at %s: %d try", strings.Join(cluster, ","), i+1)
		storage, err = NewCassandraStorage(cluster, keyspace)
		if err == nil {
			err = storage.Init()
			if err == nil {
				return storage, nil
			}
		}
		Logger.Debug("Error: %s", err)
		time.Sleep(backoff)
	}
	return nil, err
}

func NewCassandraStorage(addresses []string, keyspace string) (Storage, error) {
	cluster := gocql.NewCluster(addresses...)
	cluster.Timeout = 5 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}
	return &CassandraStorage{connection: session, keyspace: keyspace}, nil
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

func (cs *CassandraStorage) GetStack(name string) (*Stack, error) {
	stack := &Stack{}
	query := fmt.Sprintf(`SELECT name, parent, applications from %s.stacks where name = ? LIMIT 1`, cs.keyspace)
	var parent string
	var apps string
	err := cs.connection.Query(query, name).Scan(&stack.Name, &parent, &apps)
	if err != nil {
		Logger.Info("Unable get stack %s from Cassandra: %s", name, err)
		return nil, err
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

func (cs *CassandraStorage) StoreStack(stack *Stack) error {
	cs.lock.Lock()
	defer cs.lock.Unlock()

	exists, err := cs.exists(stack.Name)
	if err != nil {
		return err
	}

	if exists {
		Logger.Info("Stack %s already exists", stack.Name)
		return fmt.Errorf("Stack %s already exists", stack.Name)
	}

	if stack.From != "" {
		exists, err = cs.exists(stack.From)
		if !exists {
			Logger.Info("Parent stack %s does not exist", stack.From)
			return fmt.Errorf("Parent stack %s does not exist", stack.From)
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
		return fmt.Errorf("Stack %s does not exist", stack)
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

func (cs *CassandraStorage) Init() error {
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

	query = fmt.Sprintf("CREATE INDEX IF NOT EXISTS ON %s.stacks (parent)", cs.keyspace)
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
