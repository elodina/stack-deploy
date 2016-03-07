/* Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package framework

import "github.com/gambol99/go-marathon"

type Docker struct {
	ForcePullImage bool                 `yaml:"force_pull_image,omitempty"`
	Image          string               `yaml:"image,omitempty"`
	Network        string               `yaml:"network,omitempty"`
	Parameters     map[string]string    `yaml:"parameters,omitempty"`
	PortMappings   []*DockerPortMapping `yaml:"port_mappings,omitempty"`
	Privileged     bool                 `yaml:"privileged,omitempty"`
	Volumes        []*DockerVolume      `yaml:"volumes,omitempty"`
}

func (d *Docker) MarathonContainer() *marathon.Container {
	return &marathon.Container{
		Type: "DOCKER",
		Docker: &marathon.Docker{
			ForcePullImage: d.ForcePullImage,
			Image:          d.Image,
			Network:        d.Network,
			Parameters:     d.marathonParameters(),
			PortMappings:   d.marathonPortMappings(),
			Privileged:     d.Privileged,
		},
		Volumes: d.marathonVolumes(),
	}
}

func (d *Docker) marathonParameters() []*marathon.Parameters {
	if len(d.Parameters) == 0 {
		return nil
	}

	parameters := make([]*marathon.Parameters, 0, len(d.Parameters))
	for k, v := range d.Parameters {
		parameters = append(parameters, &marathon.Parameters{
			Key:   k,
			Value: v,
		})
	}

	return parameters
}

func (d *Docker) marathonPortMappings() []*marathon.PortMapping {
	if len(d.PortMappings) == 0 {
		return nil
	}

	mappings := make([]*marathon.PortMapping, len(d.PortMappings))
	for idx, mapping := range d.PortMappings {
		mappings[idx] = mapping.Marathon()
	}

	return mappings
}

func (d *Docker) marathonVolumes() []*marathon.Volume {
	if len(d.Volumes) == 0 {
		return nil
	}

	volumes := make([]*marathon.Volume, len(d.Volumes))
	for idx, volume := range d.Volumes {
		volumes[idx] = volume.Marathon()
	}

	return volumes
}

type DockerPortMapping struct {
	ContainerPort int    `yaml:"container_port,omitempty"`
	HostPort      int    `yaml:"host_port"`
	ServicePort   int    `yaml:"service_port,omitempty"`
	Protocol      string `yaml:"protocol"`
}

func (dpm *DockerPortMapping) Marathon() *marathon.PortMapping {
	return &marathon.PortMapping{
		ContainerPort: dpm.ContainerPort,
		HostPort:      dpm.HostPort,
		ServicePort:   dpm.ServicePort,
		Protocol:      dpm.Protocol,
	}
}

type DockerVolume struct {
	ContainerPath string `yaml:"container_path,omitempty"`
	HostPath      string `yaml:"host_path,omitempty"`
	Mode          string `yaml:"mode,omitempty"`
}

func (dv *DockerVolume) Marathon() *marathon.Volume {
	return &marathon.Volume{
		ContainerPath: dv.ContainerPath,
		HostPath:      dv.HostPath,
		Mode:          dv.Mode,
	}
}
