package store

import (
	"agentgo/types"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/influxdata/telegraf"
)

type accumulator struct {
	store *Store
}

// Accumulator returns an Accumulator compatible with our input collector.
//
// This accumlator will store point in the Store
func (s *Store) Accumulator() telegraf.Accumulator {
	return &accumulator{
		store: s,
	}
}

// AddFields adds a metric to the accumulator with the given measurement
// name, fields, and tags (and timestamp). If a timestamp is not provided,
// then the accumulator sets it to "now".
// Create a point with a value, decorating it with tags
// NOTE: tags is expected to be owned by the caller, don't mutate
// it after passing to Add.
func (a *accumulator) AddFields(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	a.addMetrics(measurement, fields, tags, t...)
}

// AddGauge is the same as AddFields, but will add the metric as a "Gauge" type
func (a *accumulator) AddGauge(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	a.addMetrics(measurement, fields, tags, t...)
}

// AddCounter is the same as AddFields, but will add the metric as a "Counter" type
func (a *accumulator) AddCounter(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	a.addMetrics(measurement, fields, tags, t...)
}

// AddSummary is the same as AddFields, but will add the metric as a "Summary" type
func (a *accumulator) AddSummary(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	a.addMetrics(measurement, fields, tags, t...)
}

// AddHistogram is the same as AddFields, but will add the metric as a "Histogram" type
func (a *accumulator) AddHistogram(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	a.addMetrics(measurement, fields, tags, t...)
}

// SetPrecision do nothing right now
func (a *accumulator) SetPrecision(precision time.Duration) {
	a.AddError(fmt.Errorf("SetPrecision not implemented"))
}

// AddMetric is not yet implemented
func (a *accumulator) AddMetric(telegraf.Metric) {
	a.AddError(fmt.Errorf("AddMetric not implemented"))
}

// WithTracking is not yet implemented
func (a *accumulator) WithTracking(maxTracked int) telegraf.TrackingAccumulator {
	a.AddError(fmt.Errorf("WithTracking not implemented"))
	return nil
}

// AddError add an error to the Accumulator
func (a *accumulator) AddError(err error) {
	if err != nil {
		log.Printf("AddError(%v)", err)
	}
}

func (a *accumulator) addMetrics(measurement string, fields map[string]interface{}, tags map[string]string, t ...time.Time) {
	labels := make(map[string]string)
	for k, v := range tags {
		labels[k] = v
	}
	var ts time.Time
	if len(t) == 1 {
		ts = t[0]
	} else {
		ts = time.Now()
	}
	a.store.lock.Lock()
	defer a.store.lock.Unlock()
	for name, value := range fields {
		labels["__name__"] = measurement + "_" + name
		value, err := convertInterface(value)
		if err != nil {
			log.Panicf("convertInterface failed. Ignoring point: %s", err)
			continue
		}
		metric := a.store.metricGetOrCreate(labels)
		a.store.addPoint(metric.metricID, types.Point{Time: ts, Value: value})
	}
}

// convertInterface convert the interface type in float64
func convertInterface(value interface{}) (float64, error) {
	switch value := value.(type) {
	case uint64:
		return float64(value), nil
	case float64:
		return value, nil
	case int:
		return float64(value), nil
	case int64:
		return float64(value), nil
	default:
		var valueType = reflect.TypeOf(value)
		return float64(0), fmt.Errorf("value type not supported :(%v)", valueType)
	}
}
