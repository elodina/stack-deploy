package utils

import (
	"strings"

	"github.com/golang/protobuf/proto"
	mesos "github.com/mesos/mesos-go/mesosproto"
)

// Convert param string like "param1=value1;param2=value2" to mesos.Labels
func StringToLabels(s string) *mesos.Labels {
	labels := &mesos.Labels{Labels: make([]*mesos.Label, 0)}
	if s == "" {
		return labels
	}
	pairs := strings.Split(s, ";")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		key, value := kv[0], kv[1]
		label := &mesos.Label{Key: proto.String(key), Value: proto.String(value)}
		labels.Labels = append(labels.Labels, label)
	}
	return labels
}
