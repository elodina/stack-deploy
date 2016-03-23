package utils

import mesos "github.com/mesos/mesos-go/mesosproto"

type TaskConfig map[string]string

func (tc TaskConfig) GetString(key string) string {
	value, ok := tc[key]
	if !ok {
		return ""
	}

	return value
}

func TaskMatches(data *TaskData, offer *mesos.Offer) string {
	if data.Cpu > GetScalarResources(offer, "cpus") {
		return "no cpus"
	}

	if data.Mem > GetScalarResources(offer, "mem") {
		return "no mem"
	}
	return ""
}
