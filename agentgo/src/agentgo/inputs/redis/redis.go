// Copyright 2015-2018 Bleemeo
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

// Package for redis input

package redis

import (
	"fmt"
	"time"

	"github.com/influxdata/telegraf"
	telegraf_inputs "github.com/influxdata/telegraf/plugins/inputs"
	"github.com/influxdata/telegraf/plugins/inputs/redis"
)

// Input countains input information about Redis
type Input struct {
	telegraf.Input
}

// New initialise redis.Input
func New(url string) *Input {
	var input, ok = telegraf_inputs.Inputs["redis"]
	if ok {
		redisInput, ok := input().(*redis.Redis)
		if ok {
			slice := append(make([]string, 0), url)
			redisInput.Servers = slice
			return &Input{redisInput}
		}
	}
	return &Input{nil}
}

// Gather takes in an accumulator and adds the metrics that the Input
// gathers. This is called every "interval"
func (input *Input) Gather(acc telegraf.Accumulator) error {
	redisAccumulator := initAccumulator(acc)
	err := input.Input.Gather(&redisAccumulator)
	return err
}

// accumulator save the redis metric from telegraf
type accumulator struct {
	acc telegraf.Accumulator
}

// InitAccumulator initialize an accumulator
func initAccumulator(acc telegraf.Accumulator) accumulator {
	return accumulator{
		acc: acc,
	}
}

// AddFields adds a metric to the accumulator with the given measurement
// name, fields, and tags (and timestamp). If a timestamp is not provided,
// then the accumulator sets it to "now".
// Create a point with a value, decorating it with tags
// NOTE: tags is expected to be owned by the caller, don't mutate
// it after passing to Add.
// nolint: gocyclo
func (accumulator *accumulator) AddFields(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	finalFields := make(map[string]interface{})
	for metricName, value := range fields {
		finalMetricName := measurement + "_" + metricName
		if finalMetricName == "redis_connected_slaves" {
			finalFields["redis_current_connections_slaves"] = value
		} else if finalMetricName == "redis_clients" {
			finalFields["redis_current_connections_clients"] = value
		} else if finalMetricName == "redis_evicted_keys" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_expired_keys" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_keyspace_hits" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_keyspace_misses" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_keyspace_hitrate" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_total_system_memory" {
			finalFields["redis_memory"] = value
		} else if finalMetricName == "redis_used_memory_lua" {
			finalFields["redis_memory_lua"] = value
		} else if finalMetricName == "redis_used_memory_peak" {
			finalFields["redis_memory_peak"] = value
		} else if finalMetricName == "redis_used_memory_rss" {
			finalFields["redis_memory_rss"] = value
		} else if finalMetricName == "redis_pubsub_channels" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_pubsub_patterns" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_total_connections_received" {
			finalFields["redis_total_connections"] = value
		} else if finalMetricName == "total_commands_processed" {
			finalFields["redis_total_operations"] = value
		} else if finalMetricName == "redis_total_commands_processed" {
			finalFields["redis_total_operations"] = value
		} else if finalMetricName == "redis_uptime" {
			finalFields[finalMetricName] = value
		} else if finalMetricName == "redis_rdb_changes_since_last_save" {
			finalFields["redis_volatile_changes"] = value
		} else {
			continue
		}
	}
	(accumulator.acc).AddFields(measurement, finalFields, nil, t...)
}

// AddError add an error to the accumulator
func (accumulator *accumulator) AddError(err error) {
	(accumulator.acc).AddError(err)
}

// This functions are useless for redis metric.
// They are not implemented

// AddGauge is useless for redis
func (accumulator *accumulator) AddGauge(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	(accumulator.acc).AddError(fmt.Errorf("AddGauge not implemented for redis accumulator"))
}

// AddCounter is useless for redis
func (accumulator *accumulator) AddCounter(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	(accumulator.acc).AddError(fmt.Errorf("AddCounter not implemented for redis accumulator"))
}

// AddSummary is useless for redis
func (accumulator *accumulator) AddSummary(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	(accumulator.acc).AddError(fmt.Errorf("AddSummary not implemented for redis accumulator"))
}

// AddHistogram is useless for redis
func (accumulator *accumulator) AddHistogram(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	(accumulator.acc).AddError(fmt.Errorf("AddHistogram not implemented for redis accumulator"))
}

// SetPrecision is useless for redis
func (accumulator *accumulator) SetPrecision(precision, interval time.Duration) {
	(accumulator.acc).AddError(fmt.Errorf("SetPrecision not implemented for redis accumulator"))
}
