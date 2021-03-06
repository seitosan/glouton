package registry

import (
	"fmt"
	"glouton/types"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
)

type QueryType int

const (
	NoProbe QueryType = iota
	OnlyProbes
	All
)

// GatherState is an argument given to gatherers that support it. It allows us to give extra informations
// to gatherers. Due to the way such objects are contructed when no argument is supplied (when calling
// Gather() on a GathererWithState, most of the time Gather() will directly call GatherWithState(GatherState{}),
// please make sure that default values are sensible. For example, NoProbe *must* be the default queryType, as
// we do not want queries on /metrics to always probe the collectors by default).
type GatherState struct {
	QueryType QueryType
	// Shall TickingGatherer perform immediately the gathering (instead of its normal "ticking"
	// operation mode) ?
	NoTick bool
}

func GatherStateFromMap(params map[string][]string) GatherState {
	state := GatherState{}

	// TODO: add this in some user-facing documentation
	if _, includeProbes := params["includeMonitors"]; includeProbes {
		state.QueryType = All
	}

	// TODO: add this in some user-facing documentation
	if _, excludeMetrics := params["onlyMonitors"]; excludeMetrics {
		state.QueryType = OnlyProbes
	}

	return state
}

// GathererWithState is a generalization of prometheus.Gather.
type GathererWithState interface {
	GatherWithState(GatherState) ([]*dto.MetricFamily, error)
}

// GathererWithStateWrapper is a wrapper around GathererWithState that allows to specify a state to forward
// to the wrapped gatherer when the caller does not know about GathererWithState and uses raw Gather().
// The main use case is the /metrics HTTP endpoint, where we want to be able to gather() only some metrics
// (e.g. all metrics/only probes/no probes).
// In the prometheus exporter endpoint, when receiving an request, the (user-provided)
// HTTP handler will:
// - create a new wrapper instance, generate a GatherState accordingly, and call wrapper.setState(newState).
// - pass the wrapper to a new prometheus HTTP handler.
// - when Gather() is called upon the wrapper by prometheus, the wrapper calls GathererWithState(newState)
// on its internal gatherer.
type GathererWithStateWrapper struct {
	gatherState GatherState
	gatherer    GathererWithState
}

// NewGathererWithStateWrapper creates a new wrapper around GathererWithState.
func NewGathererWithStateWrapper(g GathererWithState) *GathererWithStateWrapper {
	return &GathererWithStateWrapper{gatherer: g}
}

// SetState updates the state the wrapper will provide to its internal gatherer when called.
func (w *GathererWithStateWrapper) SetState(state GatherState) {
	w.gatherState = state
}

// Gather implements prometheus.Gatherer for GathererWithStateWrapper.
func (w *GathererWithStateWrapper) Gather() ([]*dto.MetricFamily, error) {
	return w.gatherer.GatherWithState(w.gatherState)
}

// labeledGatherer provide a gatherer that will add provided labels to all metrics.
// It also allow to gather to MetricPoints.
type labeledGatherer struct {
	source      prometheus.Gatherer
	labels      []*dto.LabelPair
	annotations types.MetricAnnotations
}

func newLabeledGatherer(g prometheus.Gatherer, extraLabels labels.Labels, annotations types.MetricAnnotations) labeledGatherer {
	labels := make([]*dto.LabelPair, 0, len(extraLabels))

	for _, l := range extraLabels {
		l := l
		if !strings.HasPrefix(l.Name, model.ReservedLabelPrefix) {
			labels = append(labels, &dto.LabelPair{
				Name:  &l.Name,
				Value: &l.Value,
			})
		}
	}

	return labeledGatherer{
		source:      g,
		labels:      labels,
		annotations: annotations,
	}
}

// Gather implements prometheus.Gather.
func (g labeledGatherer) Gather() ([]*dto.MetricFamily, error) {
	return g.GatherWithState(GatherState{})
}

// GatherWithState implements GathererWithState.
func (g labeledGatherer) GatherWithState(state GatherState) ([]*dto.MetricFamily, error) {
	// do not collect non-probes metrics when the user only wants probes
	if _, probe := g.source.(*ProbeGatherer); !probe && state.QueryType == OnlyProbes {
		return nil, nil
	}

	var mfs []*dto.MetricFamily

	var err error

	if cg, ok := g.source.(GathererWithState); ok {
		mfs, err = cg.GatherWithState(state)
	} else {
		mfs, err = g.source.Gather()
	}

	if len(g.labels) == 0 {
		return mfs, err
	}

	for _, mf := range mfs {
		for i, m := range mf.Metric {
			m.Label = mergeLabels(m.Label, g.labels)
			mf.Metric[i] = m
		}
	}

	return mfs, err
}

// mergeLabels merge two sorted list of labels. In case of name conflict, value from b wins.
func mergeLabels(a []*dto.LabelPair, b []*dto.LabelPair) []*dto.LabelPair {
	result := make([]*dto.LabelPair, 0, len(a)+len(b))
	aIndex := 0

	for _, bLabel := range b {
		for aIndex < len(a) && a[aIndex].GetName() < bLabel.GetName() {
			result = append(result, a[aIndex])
			aIndex++
		}

		if aIndex < len(a) && a[aIndex].GetName() == bLabel.GetName() {
			aIndex++
		}

		result = append(result, bLabel)
	}

	for aIndex < len(a) {
		result = append(result, a[aIndex])
		aIndex++
	}

	return result
}

func (g labeledGatherer) GatherPoints(state GatherState) ([]types.MetricPoint, error) {
	mfs, err := g.GatherWithState(state)
	points := familiesToMetricPoints(mfs)

	if (g.annotations != types.MetricAnnotations{}) {
		for i := range points {
			points[i].Annotations = g.annotations
		}
	}

	return points, err
}

type sliceGatherer []*dto.MetricFamily

// Gather implements Gatherer.
func (s sliceGatherer) Gather() ([]*dto.MetricFamily, error) {
	return s, nil
}

// Gatherers do the same as prometheus.Gatherers but allow different gatherer
// to have different metric help text.
// The type must still be the same.
//
// This is useful when scrapping multiple endpoints which provide the same metric
// (like "go_gc_duration_seconds") but changed its help_text.
//
// The first help_text win.
type Gatherers []prometheus.Gatherer

// GatherWithState implements GathererWithState.
func (gs Gatherers) GatherWithState(state GatherState) ([]*dto.MetricFamily, error) {
	metricFamiliesByName := map[string]*dto.MetricFamily{}

	var errs prometheus.MultiError

	var mfs []*dto.MetricFamily

	wg := sync.WaitGroup{}
	wg.Add(len(gs))

	mutex := sync.Mutex{}

	// run gather in parallel
	for _, g := range gs {
		go func(g prometheus.Gatherer) {
			defer wg.Done()

			var currentMFs []*dto.MetricFamily

			var err error

			if cg, ok := g.(GathererWithState); ok {
				currentMFs, err = cg.GatherWithState(state)
			} else {
				currentMFs, err = g.Gather()
			}

			mutex.Lock()

			if err != nil {
				errs = append(errs, err)
			}

			mfs = append(mfs, currentMFs...)

			mutex.Unlock()
		}(g)
	}

	wg.Wait()

	for _, mf := range mfs {
		existingMF, exists := metricFamiliesByName[mf.GetName()]

		if exists {
			if existingMF.GetType() != mf.GetType() {
				errs = append(errs, fmt.Errorf(
					"gathered metric family %s has type %s but should have %s",
					mf.GetName(), mf.GetType(), existingMF.GetType(),
				))

				continue
			}
		} else {
			existingMF = &dto.MetricFamily{}
			existingMF.Name = mf.Name
			existingMF.Help = mf.Help
			existingMF.Type = mf.Type
			metricFamiliesByName[mf.GetName()] = existingMF
		}

		existingMF.Metric = append(existingMF.Metric, mf.Metric...)
	}

	result := make([]*dto.MetricFamily, 0, len(metricFamiliesByName))

	for _, f := range metricFamiliesByName {
		result = append(result, f)
	}

	// Still use prometheus.Gatherers because it will:
	// * Remove (and complain) about possible duplicate of metric
	// * sort output
	// * maybe other sanity check :)

	promGatherers := prometheus.Gatherers{
		sliceGatherer(result),
	}

	sortedResult, err := promGatherers.Gather()
	if err != nil {
		if multiErr, ok := err.(prometheus.MultiError); ok {
			errs = append(errs, multiErr...)
		} else {
			errs = append(errs, err)
		}
	}

	return sortedResult, errs.MaybeUnwrap()
}

// Gather implements prometheus.Gather.
func (gs Gatherers) Gather() ([]*dto.MetricFamily, error) {
	return gs.GatherWithState(GatherState{})
}

type labeledGatherers []labeledGatherer

// GatherPoints return samples as MetricPoint instead of Prometheus MetricFamily.
func (gs labeledGatherers) GatherPoints(state GatherState) ([]types.MetricPoint, error) {
	result := []types.MetricPoint{}

	var errs prometheus.MultiError

	wg := sync.WaitGroup{}
	wg.Add(len(gs))

	mutex := sync.Mutex{}

	for _, g := range gs {
		go func(g labeledGatherer) {
			defer wg.Done()

			points, err := g.GatherPoints(state)

			mutex.Lock()

			if err != nil {
				errs = append(errs, err)
			}

			result = append(result, points...)

			mutex.Unlock()
		}(g)
	}

	wg.Wait()

	return result, errs.MaybeUnwrap()
}
