package framework

import (
	"crypto/sha1"
	"fmt"

	"encoding/json"
	"github.com/gocql/gocql"
)

type UserRole int

const (
	UserAdmin   UserRole = 0
	UserRegular UserRole = 1
)

type User struct {
	Name string   `json:"name"`
	Key  string   `json:"key"`
	Role UserRole `json:"role"`
}

type UserStorage interface {
	SaveUser(User) error
	GetUser(string) (*User, error)
	CheckKey(string, string) (bool, error)
	IsAdmin(string) (bool, error)
	CreateUser(string, UserRole) (string, error)
	RefreshToken(string) (string, error)
}

type CassandraUserStorage struct {
	connection *gocql.Session
	keyspace   string
}

func NewCassandraUserStorage(connection *gocql.Session, keyspace string) (*CassandraUserStorage, string, error) {
	store := &CassandraUserStorage{connection: connection, keyspace: keyspace}
	key, err := store.Init()
	if err != nil {
		return nil, "", err
	}
	return store, key, nil
}

func (cus CassandraUserStorage) Init() (string, error) {
	var key string
	query := cus.prepareQuery("CREATE TABLE IF NOT EXISTS %s.users (name text, key text, role int, PRIMARY KEY(name))")
	err := cus.connection.Query(query).Exec()
	if err != nil {
		return "", err
	}
	empty, err := cus.isEmpty()
	if err != nil {
		return "", err
	}
	if empty {
		key, err = cus.createAdmin()
		if err != nil {
			return "", err
		}
	}
	return key, nil
}

func (cus CassandraUserStorage) CreateUser(name string, role UserRole) (string, error) {
	exists, err := cus.userExist(name)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("User '%s' already exists", name)
	}
	key := UUID()
	user := User{Name: name,
		Role: role,
		Key:  key,
	}
	return key, cus.SaveUser(user)
}

func (cus CassandraUserStorage) RefreshToken(name string) (string, error) {
	exists, err := cus.userExist(name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("User '%s' does not exist", name)
	}
	user, err := cus.GetUser(name)
	if err != nil {
		return "", err
	}
	newKey := UUID()
	user.Key = newKey
	err = cus.SaveUser(*user)
	if err != nil {
		return "", err
	}
	return newKey, nil
}

func (cus CassandraUserStorage) SaveUser(user User) error {
	query := cus.prepareQuery("INSERT INTO %s.users (name, key, role) VALUES (?, ?, ?)")
	user.Key = sha(user.Key)
	return cus.connection.Query(query, user.Name, user.Key, user.Role).Exec()
}

func (cus CassandraUserStorage) GetUser(name string) (*User, error) {
	query := cus.prepareQuery("SELECT name, key, role FROM %s.users WHERE name = ? LIMIT 1")
	user := &User{}
	err := cus.connection.Query(query, name).Scan(&user.Name, &user.Key, &user.Role)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (cus CassandraUserStorage) CheckKey(name string, key string) (bool, error) {
	user, err := cus.GetUser(name)
	Logger.Debug("%v, %s", user, err)
	if err != nil {
		return false, err
	}
	return user.Key == sha(key), nil
}

func (cus CassandraUserStorage) IsAdmin(name string) (bool, error) {
	user, err := cus.GetUser(name)
	if err != nil {
		return false, err
	}
	return user.Role == UserAdmin, nil
}

func (cus CassandraUserStorage) createAdmin() (string, error) {
	key, err := cus.CreateUser("admin", UserAdmin)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (cus CassandraUserStorage) isEmpty() (bool, error) {
	var count int
	query := cus.prepareQuery("SELECT COUNT(*) FROM %s.users;")
	err := cus.connection.Query(query).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (cus CassandraUserStorage) userExist(name string) (bool, error) {
	var count int
	query := cus.prepareQuery("SELECT COUNT(*) FROM %s.users WHERE name = ?")
	err := cus.connection.Query(query, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (cus CassandraUserStorage) prepareQuery(query string) string {
	return fmt.Sprintf(query, cus.keyspace)
}

func sha(key string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(key)))
}

func (u *User) UnmarshalJSON(data []byte) error {
	user := struct {
		Name string `json:"name"`
		Key  string `json:"key"`
		Role string `json:"role"`
	}{}
	err := json.Unmarshal(data, &user)
	if err != nil {
		return err
	}
	u.Name = user.Name
	u.Key = user.Key
	u.Role = UserRegular
	if user.Role == "admin" {
		u.Role = UserAdmin
	}
	return nil
}
