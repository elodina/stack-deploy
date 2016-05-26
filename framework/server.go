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
	userStorage     UserStorage
	scheduler       Scheduler
}

func NewApiServer(api string, marathonClient marathon.Marathon, globalVariables map[string]string, storage Storage, userStorage UserStorage, scheduler Scheduler) *StackDeployServer {
	if strings.HasPrefix(api, "http://") {
		api = api[len("http://"):]
	}
	server := &StackDeployServer{
		api:             api,
		marathonClient:  marathonClient,
		globalVariables: globalVariables,
		storage:         storage,
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

	http.HandleFunc("/scheduled", ts.Auth(ts.ScheduledHandler))
	http.HandleFunc("/scheduled/delete", ts.Auth(ts.RemoveScheduledHandler))

	http.HandleFunc("/health", ts.HealthHandler)
	http.HandleFunc("/createuser", ts.Auth(ts.Admin(ts.CreateUserHandler)))
	http.HandleFunc("/refreshtoken", ts.Auth(ts.Admin(ts.RefreshTokenHandler)))

	http.HandleFunc("/createlayer", ts.Auth(ts.CreateLayerHandler))

	http.HandleFunc("/state", ts.Auth(ts.GetStateHandler))
	http.HandleFunc("/importstate", ts.Auth(ts.ImportStateHandler))

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

		variables := NewVariables()
		for varName, varValue := range ts.globalVariables {
			variables.SetGlobalVariable(varName, varValue)
		}
		for varName, varValue := range runRequest.Variables {
			variables.SetArbitraryVariable(varName, varValue)
		}

		context := NewRunContext(variables)
		err = ts.runStack(runRequest, context)
		if err != nil {
			Logger.Error("Run stack error: %s", err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		Logger.Info("Done running stack %s: %s", stackName, context.Variables.String())
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

func (ts *StackDeployServer) ScheduledHandler(w http.ResponseWriter, r *http.Request) {
	tasks := ts.scheduler.GetScheduledTasks()
	resp, err := json.Marshal(tasks)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func (ts *StackDeployServer) RemoveScheduledHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	var request RemoveScheduledRequest
	err := decoder.Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	removed := ts.scheduler.RemoveScheduled(request.ID)
	if !removed {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("Task %d not found", request.ID)))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%d deleted", request.ID)))
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

func (ts *StackDeployServer) GetStateHandler(w http.ResponseWriter, r *http.Request) {
	Logger.Debug("Received get state command")
	defer r.Body.Close()

	state, err := ts.storage.GetState()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	jsonState, err := json.Marshal(state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonState)
}

func (ts *StackDeployServer) ImportStateHandler(w http.ResponseWriter, r *http.Request) {
	Logger.Debug("Received import state command")
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	var stringState string
	err := decoder.Decode(&stringState)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var state *StackDeployState
	err = json.Unmarshal([]byte(stringState), &state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = ts.restoreState(state)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (ts *StackDeployServer) runStack(request *RunRequest, context *RunContext) error {
	runner, err := ts.storage.GetStackRunner(request.Name)
	if err != nil {
		return err
	}
	if request.Zone != "" {
		layers, err := ts.storage.GetLayersStack(request.Zone)
		if err != nil {
			return err
		}
		layers.Merge(runner.GetStack())
		runner = layers.GetRunner()
	}

	Logger.Info("Running stack %s in zone '%s' and variables %s", request.Name, request.Zone, context.Variables)
	context.StackName = request.Name
	context.Zone = request.Zone
	context.Marathon = ts.marathonClient
	context.Scheduler = ts.scheduler
	context.Storage = ts.storage
	err = context.Storage.SaveStackStatus(context.StackName, context.Zone, StackStatusStaging)
	if err != nil {
		return err
	}

	err = runner.Run(request, context)
	if err != nil {
		_ = context.Storage.SaveStackStatus(context.StackName, context.Zone, StackStatusFailed)
	} else {
		err = context.Storage.SaveStackStatus(context.StackName, context.Zone, StackStatusRunning)
	}
	return err
}

func (ts *StackDeployServer) restoreState(state *StackDeployState) error {
	err := ts.addStacks(state.GetStacks(), make(map[string]struct{}))
	if err != nil {
		return err
	}

	stacks := StackStateSlice(state.RunningStacks)
	// we should run stacks in the same order as we did initially
	sort.Sort(stacks)

	runningStacks := make([]*StackState, 0)
	// also filter only running stacks
	for _, stackState := range stacks {
		if stackState.Status == StackStatusRunning {
			runningStacks = append(runningStacks, stackState)
		}
	}

	// now run these stacks in a given order
	for _, stackState := range runningStacks {
		runRequest := NewRunRequest()
		runRequest.Name = stackState.Name
		runRequest.Zone = stackState.Zone

		variables := NewVariables()
		for varName, varValue := range ts.globalVariables {
			variables.SetGlobalVariable(varName, varValue)
		}
		context := NewRunContext(variables)
		err := ts.runStack(runRequest, context)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ts *StackDeployServer) addStacks(stacks []*Stack, addedStacks map[string]struct{}) error {
	if len(stacks) == 0 {
		return nil
	}

	var addedAtLeastOnce bool
	childStacks := make([]*Stack, 0)
	for _, stack := range stacks {
		_, exists := addedStacks[stack.From]
		if stack.From == "" || exists {
			addedAtLeastOnce = true
			err := ts.storage.StoreStack(stack)
			if err != nil {
				return err
			}
			addedStacks[stack.Name] = struct{}{}
		} else {
			childStacks = append(childStacks, stack)
		}
	}

	if !addedAtLeastOnce {
		return fmt.Errorf("Orphan stack '%s' detected, preventing infinite loop", stacks[0].Name)
	}

	return ts.addStacks(childStacks, addedStacks)
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
