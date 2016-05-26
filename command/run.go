package command

import (
	"fmt"

	"flag"
	"time"

	"errors"
	api "github.com/elodina/stack-deploy/framework"
	"regexp"
	"strings"
)

type vars map[string]string

func (v vars) String() string {
	stringVars := make([]string, len(v))
	idx := 0
	for k, v := range v {
		stringVars[idx] = fmt.Sprintf("%s=%s", k, v)
		idx++
	}

	return strings.Join(stringVars, ", ")
}

func (v vars) Set(value string) error {
	kv := strings.SplitN(value, "=", 2)
	if len(kv) != 2 {
		return errors.New("Variables are expected in k=v format.")
	}

	v[kv[0]] = kv[1]
	return nil
}

type skipApplications []string

func (s *skipApplications) String() string {
	return strings.Join(*s, ", ")
}

func (s *skipApplications) Set(value string) error {
	_, err := regexp.Compile(value)
	if err != nil {
		return err
	}

	*s = append(*s, value)
	return nil
}

type RunCommand struct{}

func (rc *RunCommand) Run(args []string) int {
	if len(args) == 0 {
		fmt.Println("Stack name required to run")
		return 1
	}

	var (
		flags            = flag.NewFlagSet("run", flag.ExitOnError)
		apiUrl           = flags.String("api", "", "Stack-deploy server address.")
		zone             = flags.String("zone", "", "Zone to run stack.")
		maxWait          = flags.Int("max.wait", api.DefaultApplicationMaxWait, "Maximum time to wait for each application in a stack to become running and healthy.")
		variables        = make(vars)
		skipApplications = make(skipApplications, 0)
	)
	flags.Var(variables, "var", "Arbitrary variables to add to stack context. Multiple occurrences of this flag allowed.")
	flags.Var(&skipApplications, "skip", "Regular expression of applications to skip in stack. Multiple occurrences of this flag allowed.")
	flags.Parse(args[1:])

	name := args[0]
	stackDeployApi, err := resolveApi(*apiUrl)
	if err != nil {
		fmt.Printf("ERROR resolving API: %s\n", err)
		return 1
	}
	client := api.NewClient(stackDeployApi)

	fmt.Printf("Running stack %s\n", name)
	start := time.Now()
	err = client.Run(&api.RunRequest{
		Name:             name,
		Zone:             *zone,
		MaxWait:          *maxWait,
		Variables:        variables,
		SkipApplications: skipApplications,
	})
	if err != nil {
		fmt.Printf("ERROR running client request: %s\n", err)
		return 1
	}

	elapsed := time.Since(start)
	fmt.Printf("Done in %s\n", elapsed-elapsed%time.Second)
	return 0
}

func (rc *RunCommand) Help() string {
	return ""
}

func (rc *RunCommand) Synopsis() string {
	return "Run Stack"
}
