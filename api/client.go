package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/elodina/pyrgus/log"
	"github.com/elodina/stack-deploy/framework"
	yaml "gopkg.in/yaml.v2"
)

var Logger = log.NewDefaultLogger()

type requestData map[string]string

type apiRequest struct {
	url  string
	data map[string]string
}

type Client struct {
	host string
	user string
	key  string
}

func NewClient(host string) *Client {
	if !strings.HasPrefix(host, "http://") {
		host = "http://" + host
	}
	user := os.Getenv("SD_USER")
	key := os.Getenv("SD_KEY")
	if user == "" || key == "" {
		fmt.Println("Warning: Environment SD_USER or SD_KEY is empty")
	}
	return &Client{host: host, user: user, key: key}
}

func (c *Client) Ping() error {
	request := apiRequest{
		url:  "/health",
		data: requestData{},
	}
	_, err := c.request(request)
	if err != nil {
		return err
	}
	fmt.Println("Pong")
	return nil
}

func (c *Client) List() ([]string, error) {
	request := apiRequest{
		url:  "/list",
		data: requestData{},
	}
	content, err := c.request(request)
	if err != nil {
		return nil, err
	}

	stacks := make([]string, 0)
	err = yaml.Unmarshal(content, &stacks)
	if err != nil {
		return nil, err
	}

	return stacks, err
}

func (c *Client) GetStack(name string) (*framework.Stack, error) {
	request := apiRequest{
		url:  "/get",
		data: requestData{"name": name},
	}
	content, err := c.request(request)
	if err != nil {
		return nil, err
	}

	stack := new(framework.Stack)
	err = yaml.Unmarshal(content, &stack)
	if err != nil {
		return nil, err
	}

	return stack, err
}

func (c *Client) Run(name string, zone string) error {
	request := apiRequest{
		url:  "/run",
		data: requestData{"name": name, "zone": zone},
	}
	_, err := c.request(request)
	return err
}

func (c *Client) CreateStack(stackData string) error {
	request := apiRequest{
		url:  "/createstack",
		data: requestData{"stackfile": stackData},
	}
	_, err := c.request(request)
	return err
}

func (c *Client) RemoveStack(name string, force bool) error {
	request := apiRequest{
		url:  "/removestack",
		data: requestData{"name": name, "force": fmt.Sprint(force)},
	}
	_, err := c.request(request)
	return err
}

func (c *Client) CreateUser(name string, admin bool) (string, error) {
	role := "regular"
	if admin {
		role = "admin"
	}
	request := apiRequest{
		url:  "/createuser",
		data: requestData{"name": name, "role": role},
	}
	resp, err := c.request(request)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Client) RefreshToken(name string) (string, error) {
	request := apiRequest{
		url:  "/refreshtoken",
		data: requestData{"name": name},
	}
	resp, err := c.request(request)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Client) request(request apiRequest) ([]byte, error) {
	Logger.Debug("Sending request: %v", request)
	jsonData, err := json.Marshal(request.data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.host+request.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-Api-User", c.user)
	req.Header.Add("X-Api-Key", c.key)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		Logger.Debug("Error sending post request: %s", err)
		return nil, err
	}

	defer resp.Body.Close()
	Logger.Debug("Status code: %d", resp.StatusCode)
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Logger.Critical("Can't read response body: %s", err)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		Logger.Debug("Status code is not 200: %d", resp.StatusCode)
		return nil, errors.New(string(content))
	}

	return content, nil
}
