package framework

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type apiRequest struct {
	url  string
	data interface{}
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
		data: make(map[string]interface{}),
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
		data: make(map[string]interface{}),
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

func (c *Client) GetStack(data *GetStackRequest) (*Stack, error) {
	request := apiRequest{
		url:  "/get",
		data: data,
	}
	content, err := c.request(request)
	if err != nil {
		return nil, err
	}

	stack := new(Stack)
	err = yaml.Unmarshal(content, &stack)
	if err != nil {
		return nil, err
	}

	return stack, err
}

func (c *Client) Run(data *RunRequest) error {
	request := apiRequest{
		url:  "/run",
		data: data,
	}
	_, err := c.request(request)
	return err
}

func (c *Client) CreateStack(data *CreateStackRequest) error {
	request := apiRequest{
		url:  "/createstack",
		data: data,
	}
	_, err := c.request(request)
	return err
}

func (c *Client) CreateLayer(data *CreateLayerRequest) error {
	request := apiRequest{
		url:  "/createlayer",
		data: data,
	}
	_, err := c.request(request)
	return err
}

func (c *Client) RemoveStack(data *RemoveStackRequest) error {
	request := apiRequest{
		url:  "/removestack",
		data: data,
	}
	_, err := c.request(request)
	return err
}

func (c *Client) CreateUser(data *CreateUserRequest) (string, error) {
	request := apiRequest{
		url:  "/createuser",
		data: data,
	}
	resp, err := c.request(request)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Client) RefreshToken(data *RefreshTokenRequest) (string, error) {
	request := apiRequest{
		url:  "/refreshtoken",
		data: data,
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
