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

package threshold

import (
	"context"
	"fmt"
	"glouton/logger"
	"glouton/types"
	"math"
	"sync"
	"time"
)

const statusCacheKey = "CacheStatusState"

// State store information about current firing threshold.
type State interface {
	Get(key string, result interface{}) error
	Set(key string, object interface{}) error
}

// Registry keep track of threshold states to update metrics if the exceed a threshold for a period
// Use WithPusher() to create a pusher and sent metric points to it.
type Registry struct {
	state State

	l                 sync.Mutex
	states            map[MetricNameItem]statusState
	units             map[MetricNameItem]Unit
	thresholdsAllItem map[string]Threshold
	thresholds        map[MetricNameItem]Threshold
	defaultSoftPeriod time.Duration
	softPeriods       map[string]time.Duration
}

// New returns a new ThresholdState.
func New(state State) *Registry {
	self := &Registry{
		state:             state,
		states:            make(map[MetricNameItem]statusState),
		defaultSoftPeriod: 300 * time.Second,
	}

	var jsonList []jsonState

	err := state.Get(statusCacheKey, &jsonList)
	if err != nil {
		for _, v := range jsonList {
			self.states[v.MetricNameItem] = v.statusState
		}
	}

	return self
}

// SetThresholds configure thresholds.
// The thresholdWithItem is searched first and only match if both the metric name and item match
// Then thresholdAllItem is searched and match is the metric name match regardless of the metric item.
func (r *Registry) SetThresholds(thresholdWithItem map[MetricNameItem]Threshold, thresholdAllItem map[string]Threshold) {
	r.l.Lock()
	defer r.l.Unlock()

	r.thresholdsAllItem = thresholdAllItem
	r.thresholds = thresholdWithItem

	logger.V(2).Printf("Thresholds contains %d definitions for specific item and %d definitions for any item", len(thresholdWithItem), len(thresholdAllItem))
}

// SetSoftPeriod configure soft status period. A metric must stay in higher status for at least this period before its status actually change.
// For example, CPU usage must be above 80% for at least 5 minutes before being alerted. The term soft-status is taken from Nagios.
func (r *Registry) SetSoftPeriod(defaultPeriod time.Duration, periodPerMetrics map[string]time.Duration) {
	r.l.Lock()
	defer r.l.Unlock()

	r.softPeriods = periodPerMetrics
	r.defaultSoftPeriod = defaultPeriod

	logger.V(2).Printf("SoftPeriod contains %d definitions", len(periodPerMetrics))
}

// SetUnits configure the units.
func (r *Registry) SetUnits(units map[MetricNameItem]Unit) {
	r.l.Lock()
	defer r.l.Unlock()

	r.units = units

	logger.V(2).Printf("Units contains %d definitions", len(units))
}

// MetricNameItem is the couple Name and Item.
type MetricNameItem struct {
	Name string
	Item string
}

type statusState struct {
	CurrentStatus types.Status
	CriticalSince time.Time
	WarningSince  time.Time
	LastUpdate    time.Time
}

type jsonState struct {
	MetricNameItem
	statusState
}

func (s statusState) Update(newStatus types.Status, period time.Duration, now time.Time) statusState {
	if s.CurrentStatus == types.StatusUnset {
		s.CurrentStatus = newStatus
	}

	// Make sure time didn't jump backward.
	if s.CriticalSince.After(now) {
		s.CriticalSince = time.Time{}
	}

	if s.WarningSince.After(now) {
		s.WarningSince = time.Time{}
	}

	criticalDuration := time.Duration(0)
	warningDuration := time.Duration(0)

	switch newStatus {
	case types.StatusCritical:
		if s.CriticalSince.IsZero() {
			s.CriticalSince = now
		}

		if s.WarningSince.IsZero() {
			s.WarningSince = now
		}

		criticalDuration = now.Sub(s.CriticalSince)
		warningDuration = now.Sub(s.WarningSince)
	case types.StatusWarning:
		s.CriticalSince = time.Time{}

		if s.WarningSince.IsZero() {
			s.WarningSince = now
		}

		warningDuration = now.Sub(s.WarningSince)
	default:
		s.CriticalSince = time.Time{}
		s.WarningSince = time.Time{}
	}

	switch {
	case period == 0:
		s.CurrentStatus = newStatus
	case criticalDuration >= period:
		s.CurrentStatus = types.StatusCritical
	case warningDuration >= period:
		s.CurrentStatus = types.StatusWarning
	case s.CurrentStatus == types.StatusCritical && newStatus == types.StatusWarning:
		// downgrade status immediately
		s.CurrentStatus = types.StatusWarning
	case newStatus == types.StatusOk:
		// downgrade status immediately
		s.CurrentStatus = types.StatusOk
	}

	s.LastUpdate = time.Now()

	return s
}

// Threshold define a min/max thresholds.
// Use NaN to mark the limit as unset.
type Threshold struct {
	LowCritical  float64
	LowWarning   float64
	HighWarning  float64
	HighCritical float64
}

// Equal test equality of threhold object.
func (t Threshold) Equal(other Threshold) bool {
	if t == other {
		return true
	}
	// Need special handling for NaN
	if t.LowCritical != other.LowCritical && (!math.IsNaN(t.LowCritical) || !math.IsNaN(other.LowCritical)) {
		return false
	}

	if t.LowWarning != other.LowWarning && (!math.IsNaN(t.LowWarning) || !math.IsNaN(other.LowWarning)) {
		return false
	}

	if t.HighWarning != other.HighWarning && (!math.IsNaN(t.HighWarning) || !math.IsNaN(other.HighWarning)) {
		return false
	}

	if t.HighCritical != other.HighCritical && (!math.IsNaN(t.HighCritical) || !math.IsNaN(other.HighCritical)) {
		return false
	}

	return true
}

// Unit represent the unit of a metric.
type Unit struct {
	UnitType int    `json:"unit,omitempty"`
	UnitText string `json:"unit_text,omitempty"`
}

// Possible value for UnitType.
const (
	UnitTypeUnit = 0
	UnitTypeByte = 2
	UnitTypeBit  = 3
)

// FromInterfaceMap convert a map[string]interface{} to Threshold.
// It expect the key "low_critical", "low_warning", "high_critical" and "high_warning".
func FromInterfaceMap(input map[string]interface{}) (Threshold, error) {
	result := Threshold{
		LowCritical:  math.NaN(),
		LowWarning:   math.NaN(),
		HighWarning:  math.NaN(),
		HighCritical: math.NaN(),
	}

	for _, name := range []string{"low_critical", "low_warning", "high_warning", "high_critical"} {
		if raw, ok := input[name]; ok {
			var value float64

			switch v := raw.(type) {
			case float64:
				value = v
			case int:
				value = float64(v)
			default:
				return result, fmt.Errorf("%v is not a float", raw)
			}

			switch name {
			case "low_critical":
				result.LowCritical = value
			case "low_warning":
				result.LowWarning = value
			case "high_warning":
				result.HighWarning = value
			case "high_critical":
				result.HighCritical = value
			}
		}
	}

	return result, nil
}

// IsZero returns true is all threshold limit are unset (NaN).
// Is also returns true is all threshold are equal and 0 (which is the zero-value of Threshold structure
// and is an invalid threshold configuration).
func (t Threshold) IsZero() bool {
	if math.IsNaN(t.LowCritical) && math.IsNaN(t.LowWarning) && math.IsNaN(t.HighWarning) && math.IsNaN(t.HighCritical) {
		return true
	}

	return t.LowCritical == 0.0 && t.LowWarning == 0.0 && t.HighWarning == 0.0 && t.HighCritical == 0.0
}

// CurrentStatus returns the current status regarding the threshold and (if not ok) return the exceeded limit.
func (t Threshold) CurrentStatus(value float64) (types.Status, float64) {
	if !math.IsNaN(t.LowCritical) && value < t.LowCritical {
		return types.StatusCritical, t.LowCritical
	}

	if !math.IsNaN(t.LowWarning) && value < t.LowWarning {
		return types.StatusWarning, t.LowWarning
	}

	if !math.IsNaN(t.HighCritical) && value > t.HighCritical {
		return types.StatusCritical, t.HighCritical
	}

	if !math.IsNaN(t.HighWarning) && value > t.HighWarning {
		return types.StatusWarning, t.HighWarning
	}

	return types.StatusOk, math.NaN()
}

// GetThreshold return the current threshold for given Metric.
func (r *Registry) GetThreshold(key MetricNameItem) Threshold {
	r.l.Lock()
	defer r.l.Unlock()

	return r.getThreshold(key)
}

func (r *Registry) getThreshold(key MetricNameItem) Threshold {
	if threshold, ok := r.thresholds[key]; ok {
		return threshold
	}

	v := r.thresholdsAllItem[key.Name]
	if v.IsZero() {
		return Threshold{
			LowCritical:  math.NaN(),
			LowWarning:   math.NaN(),
			HighWarning:  math.NaN(),
			HighCritical: math.NaN(),
		}
	}

	return v
}

// Run will periodically save status state and clean it.
func (r *Registry) Run(ctx context.Context) error {
	lastSave := time.Now()

	for ctx.Err() == nil {
		save := false

		select {
		case <-ctx.Done():
			save = true
		case <-time.After(time.Minute):
		}

		if time.Since(lastSave) > 60*time.Minute {
			save = true
		}

		r.run(save)

		if save {
			lastSave = time.Now()
		}
	}

	return nil
}

func (r *Registry) run(save bool) {
	r.l.Lock()
	defer r.l.Unlock()

	jsonList := make([]jsonState, 0, len(r.states))

	for k, v := range r.states {
		if time.Since(v.LastUpdate) > 60*time.Minute {
			delete(r.states, k)
		} else {
			jsonList = append(jsonList, jsonState{
				MetricNameItem: k,
				statusState:    v,
			})
		}
	}

	if save {
		_ = r.state.Set(statusCacheKey, jsonList)
	}
}

func formatValue(value float64, unit Unit) string {
	switch unit.UnitType {
	case UnitTypeUnit:
		return fmt.Sprintf("%.2f", value)
	case UnitTypeBit, UnitTypeByte:
		scales := []string{"", "K", "M", "G", "T", "P", "E"}

		i := 0
		for i < len(scales)-1 && math.Abs(value) >= 1024 {
			i++

			value /= 1024
		}

		return fmt.Sprintf("%.2f %s%ss", value, scales[i], unit.UnitText)
	default:
		return fmt.Sprintf("%.2f %s", value, unit.UnitText)
	}
}

func formatDuration(period time.Duration) string {
	if period <= 0 {
		return ""
	}

	units := []struct {
		Scale float64
		Name  string
	}{
		{1, "second"},
		{60, "minute"},
		{60, "hour"},
		{24, "day"},
	}

	currentUnit := ""
	value := period.Seconds()

	for _, unit := range units {
		if math.Round(value/unit.Scale) >= 1 {
			value /= unit.Scale
			currentUnit = unit.Name
		} else {
			break
		}
	}

	value = math.Round(value)
	if value > 1 {
		currentUnit += "s"
	}

	return fmt.Sprintf("%.0f %s", value, currentUnit)
}

type pusher struct {
	registry *Registry
	pusher   types.PointPusher
}

// WithPusher return the same threshold instance with specified point pusher.
func (r *Registry) WithPusher(p types.PointPusher) types.PointPusher {
	return pusher{
		registry: r,
		pusher:   p,
	}
}

// PushPoints implement PointPusher and do threshold.
func (p pusher) PushPoints(points []types.MetricPoint) {
	p.registry.l.Lock()

	result := make([]types.MetricPoint, 0, len(points))

	for _, point := range points {
		if !point.Annotations.Status.CurrentStatus.IsSet() {
			key := MetricNameItem{
				Name: point.Labels[types.LabelName],
				Item: point.Annotations.BleemeoItem,
			}

			threshold := p.registry.getThreshold(key)
			if !threshold.IsZero() {
				result = p.addPointWithThreshold(result, point, threshold, key)
				continue
			}
		}

		result = append(result, point)
	}

	p.registry.l.Unlock()
	p.pusher.PushPoints(result)
}

func (p *pusher) addPointWithThreshold(points []types.MetricPoint, point types.MetricPoint, threshold Threshold, key MetricNameItem) []types.MetricPoint {
	softStatus, thresholdLimit := threshold.CurrentStatus(point.Value)
	previousState := p.registry.states[key]
	period := p.registry.defaultSoftPeriod

	if tmp, ok := p.registry.softPeriods[key.Name]; ok {
		period = tmp
	}

	newState := previousState.Update(softStatus, period, time.Now())
	p.registry.states[key] = newState

	unit := p.registry.units[key]
	// Consumer expect status description from threshold to start with "Current value:"
	statusDescription := fmt.Sprintf("Current value: %s", formatValue(point.Value, unit))

	if newState.CurrentStatus != types.StatusOk {
		if period > 0 {
			statusDescription += fmt.Sprintf(
				" threshold (%s) exceeded over last %v",
				formatValue(thresholdLimit, unit),
				formatDuration(period),
			)
		} else {
			statusDescription += fmt.Sprintf(
				" threshold (%s) exceeded",
				formatValue(thresholdLimit, unit),
			)
		}
	}

	status := types.StatusDescription{
		CurrentStatus:     newState.CurrentStatus,
		StatusDescription: statusDescription,
	}
	annotationsCopy := point.Annotations
	annotationsCopy.Status = status

	points = append(points, types.MetricPoint{
		Point:       point.Point,
		Labels:      point.Labels,
		Annotations: annotationsCopy,
	})

	labelsCopy := make(map[string]string, len(point.Labels))

	for k, v := range point.Labels {
		labelsCopy[k] = v
	}

	labelsCopy[types.LabelName] += "_status"

	annotationsCopy.StatusOf = point.Labels[types.LabelName]

	points = append(points, types.MetricPoint{
		Point:       types.Point{Time: point.Time, Value: float64(status.CurrentStatus.NagiosCode())},
		Labels:      labelsCopy,
		Annotations: annotationsCopy,
	})

	return points
}
