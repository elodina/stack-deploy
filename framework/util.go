package framework

import "github.com/elodina/pyrgus/log"

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
