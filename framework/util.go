package framework

import (
	"fmt"
	"github.com/elodina/pyrgus/log"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
)

var Logger = log.NewDefaultLogger()

func setStringSlice(from []string, to *[]string) {
	if len(from) > 0 {
		*to = from
	}
}

func setIntSlice(from []int, to *[]int) {
	if len(from) > 0 {
		*to = from
	}
}

func setString(from string, to *string) {
	if from != "" {
		*to = from
	}
}

func setFloat(from float64, to *float64) {
	if from != 0.0 {
		*to = from
	}
}

func MapSliceToMap(slice yaml.MapSlice) map[string]string {
	m := make(map[string]string)
	for _, entry := range slice {
		m[fmt.Sprint(entry.Key)] = fmt.Sprint(entry.Value)
	}

	return m
}

func UUID() string {
	return uuid.NewV4().String()
}
