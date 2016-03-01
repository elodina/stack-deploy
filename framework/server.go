package framework

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	marathon "github.com/gambol99/go-marathon"
	yaml "gopkg.in/yaml.v2"
)

type StackDeployServer struct {
	api             string
	marathonClient  marathon.Marathon
	globalVariables map[string]string
	storage         Storage
	stateStorage    StateStorage
	userStorage     UserStorage
	scheduler       Scheduler
}

func NewApiServer(api string, marathonClient marathon.Marathon, globalVariables map[string]string, storage Storage, userStorage UserStorage, stateStorage StateStorage,
	scheduler Scheduler) *StackDeployServer {
	if strings.HasPrefix(api, "http://") {
		api = api[len("http://"):]
	}
	server := &StackDeployServer{
		api:             api,
		marathonClient:  marathonClient,
		globalVariables: globalVariables,
		storage:         storage,
		stateStorage:    stateStorage,
		userStorage:     userStorage,
		scheduler:       scheduler,
	}
	return server
}

func (ts *StackDeployServer) Start() {
	http.HandleFunc("/list", ts.Auth(ts.ListHandler))
	http.HandleFunc("/get", ts.Auth(ts.GetStackHandler))
	http.HandleFunc("/run", ts.Auth(ts.RunHandler))
	http.HandleFunc("/createstack", ts.Auth(ts.CreateStackHandler))
	http.HandleFunc("/removestack", ts.Auth(ts.RemoveStackHandler))
	http.HandleFunc("/health", ts.HealthHandler)
	http.HandleFunc("/createuser", ts.Auth(ts.Admin(ts.CreateUserHandler)))
	http.HandleFunc("/refreshtoken", ts.Auth(ts.Admin(ts.RefreshTokenHandler)))

	http.HandleFunc("/createlayer", ts.Auth(ts.CreateLayerHandler))

	err := ts.scheduler.Start()
	if err != nil {
		panic(err)
	}

	Logger.Info("Start API Server on: %s", ts.api)
	err = http.ListenAndServe(ts.api, nil)
	if err != nil {
		panic(err)
	}
}

// Middleware for authentication check
func (ts *StackDeployServer) Auth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Api-User")
		key := r.Header.Get("X-Api-Key")
		Logger.Debug("User %s, key %s", user, key)
		valid, err := ts.userStorage.CheckKey(user, key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !valid {
			http.Error(w, "Unauthorized", http.StatusForbidden)
			return
		}
		handler(w, r)
	}
}

// Middleware for admin role check
func (ts *StackDeployServer) Admin(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Api-User")
		admin, err := ts.userStorage.IsAdmin(user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !admin {
			http.Error(w, fmt.Sprintf("User %s is not an admin", user), http.StatusForbidden)
			return
		}
		handler(w, r)
	}
}

func (ts *StackDeployServer) ListHandler(w http.ResponseWriter, r *http.Request) {
	Logger.Debug("Received list command")
	defer r.Body.Close()

	stacks, err := ts.storage.GetAll()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	stackNames := make([]string, len(stacks))
	for idx, stack := range stacks {
		stackNames[idx] = stack.Name
	}

	sort.Strings(stackNames)
	yamlStacks, err := yaml.Marshal(stackNames)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(yamlStacks)
}

func (ts *StackDeployServer) GetStackHandler(w http.ResponseWriter, r *http.Request) {
	Logger.Debug("Received get stack command")
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	getRequest := new(GetStackRequest)
	decoder.Decode(&getRequest)
	if getRequest.Name == "" {
		http.Error(w, "Stack name required", http.StatusBadRequest)
		return
	} else {
		stack, err := ts.storage.GetStack(getRequest.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		yamlStack, err := yaml.Marshal(stack)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(yamlStack)
	}
}

func (ts *StackDeployServer) RunHandler(w http.ResponseWriter, r *http.Request) {
	Logger.Debug("Received run command")
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	runRequest := new(RunRequest)
	decoder.Decode(&runRequest)
	Logger.Debug("Run request: %#v", runRequest)
	stackName := runRequest.Name
	if stackName == "" {
		http.Error(w, "Stack name required", http.StatusBadRequest)
		return
	} else {
		//refresh Mesos state first, consider refreshing periodically when supporting auto-scaling
		err := ts.scheduler.GetMesosState().Update()
		if err != nil {
			Logger.Error("Refresh Mesos state error: %s", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		context := NewContext()
		for varName, varValue := range ts.globalVariables {
			context.SetGlobalVariable(varName, varValue)
		}
		for varName, varValue := range runRequest.Variables {
			context.SetArbitraryVariable(varName, varValue)
		}

		_, err = ts.runStack(runRequest, context, ts.storage)
		if err != nil {
			Logger.Error("Run stack error: %s", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) CreateStackHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	request := new(CreateStackRequest)
	err := decoder.Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stack, err := UnmarshalStack([]byte(request.Stackfile))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	Logger.Debug(stack)
	err = ts.storage.StoreStack(stack)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) CreateLayerHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	request := new(CreateLayerRequest)
	err := decoder.Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stack, err := UnmarshalStack([]byte(request.Stackfile))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stack.Layer, err = layerToInt(request.Layer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stack.From = request.Parent
	err = ts.storage.StoreStack(stack)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) RemoveStackHandler(w http.ResponseWriter, r *http.Request) {
	Logger.Debug("Received remove command")
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	removeRequest := new(RemoveStackRequest)
	decoder.Decode(&removeRequest)

	stackName := removeRequest.Name
	if stackName == "" {
		http.Error(w, "Stack name required", http.StatusBadRequest)
		return
	} else {
		err := ts.storage.RemoveStack(stackName, removeRequest.Force)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	var user User
	err := decoder.Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	Logger.Debug("Creating user: %v", user)
	key, err := ts.userStorage.CreateUser(user.Name, user.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(key))
}

func (ts *StackDeployServer) RefreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	var user User
	err := decoder.Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	key, err := ts.userStorage.RefreshToken(user.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(key))
}

func (ts *StackDeployServer) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) runStack(request *RunRequest, context *StackContext, storage Storage) (*StackContext, error) {
	runner, err := storage.GetStackRunner(request.Name)
	if err != nil {
		return nil, err
	}
	if request.Zone != "" {
		layers, err := storage.GetLayersStack(request.Zone)
		if err != nil {
			return nil, err
		}
		layers.Merge(runner.GetStack())
		runner = layers.GetRunner()
	}

	Logger.Info("Running stack %s in zone '%s' and context %s", request.Name, request.Zone, context)
	return runner.Run(request, context, ts.marathonClient, ts.scheduler, ts.stateStorage)
}

func layerToInt(layer string) (int, error) {
	switch layer {
	case "zone":
		return LayerZone, nil
	case "cluster":
		return LayerCluster, nil
	case "datacenter":
		return LayerDataCenter, nil
	}
	return 0, fmt.Errorf("Invalid layer: %s", layer)
}
