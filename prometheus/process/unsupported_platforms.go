// Copyright 2015-2019 Bleemeo
//
// bleemeo.com an infrastructure monitoring solution in the Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !linux

package process

import (
	"time"

	"glouton/discovery"
	"glouton/facts"
	"glouton/prometheus/registry"
)

// RegisterExporter does nothing, process_exporter is not supported on this platform.
func RegisterExporter(reg *registry.Registry, psLister interface{}, processQuerier *discovery.DynamicDiscovery, bleemeoFormat bool) {
}

func NewProcessLister(hostRootPath string, defaultValidity time.Duration) facts.ProcessLister {
	return nil
}