package discovery

import (
	"agentgo/logger"
	"agentgo/task"
	"agentgo/types"
	"context"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
)

// Accumulator will gather metrics point for added checks
type Accumulator interface {
	AddFieldsWithStatus(measurement string, fields map[string]interface{}, tags map[string]string, statuses map[string]types.StatusDescription, createStatusOf bool, t ...time.Time)
}

// Discovery implement the full discovery mecanisme. It will take informations
// from both the dynamic discovery (service currently running) and previously
// detected services.
// It will configure metrics input and add them to a Collector
type Discovery struct {
	l sync.Mutex

	dynamicDiscovery Discoverer

	servicesMap         map[nameContainer]Service
	lastDiscoveryUpdate time.Time

	acc                   Accumulator
	lastConfigservicesMap map[nameContainer]Service
	activeInput           map[nameContainer]int
	activeCheck           map[nameContainer]int
	coll                  Collector
	taskRegistry          Registry
}

// Collector will gather metrics for added inputs
type Collector interface {
	AddInput(input telegraf.Input, shortName string) int
	RemoveInput(int)
}

// Registry will contains checks
type Registry interface {
	AddTask(task task.Runner, shortName string) (int, error)
	RemoveTask(int)
}

// New returns a new Discovery
func New(dynamicDiscovery Discoverer, coll Collector, taskRegistry Registry, initialServices []Service, acc Accumulator) *Discovery {
	servicesMap := make(map[nameContainer]Service, len(initialServices))
	for _, v := range initialServices {
		key := nameContainer{
			name:        v.Name,
			containerID: v.ContainerID,
		}
		servicesMap[key] = v
	}
	return &Discovery{
		dynamicDiscovery: dynamicDiscovery,
		servicesMap:      servicesMap,
		coll:             coll,
		taskRegistry:     taskRegistry,
		acc:              acc,
		activeInput:      make(map[nameContainer]int),
		activeCheck:      make(map[nameContainer]int),
	}
}

// Close stop & cleanup inputs & check created by the discovery
func (d *Discovery) Close() {
	d.l.Lock()
	defer d.l.Unlock()
	_ = d.configureMetricInputs(d.servicesMap, nil)
	d.configureChecks(d.servicesMap, nil)
}

// Discovery detect service on the system and return a list of Service object.
//
// It may trigger an update of metric inputs present in the Collector
func (d *Discovery) Discovery(ctx context.Context, maxAge time.Duration) (services []Service, err error) {
	d.l.Lock()
	defer d.l.Unlock()
	return d.discovery(ctx, maxAge)
}

// LastUpdate return when the last update occurred
func (d *Discovery) LastUpdate() time.Time {
	d.l.Lock()
	defer d.l.Unlock()
	return d.lastDiscoveryUpdate
}

func (d *Discovery) discovery(ctx context.Context, maxAge time.Duration) (services []Service, err error) {

	if time.Since(d.lastDiscoveryUpdate) > maxAge {
		err := d.updateDiscovery(ctx, maxAge)
		if err != nil {
			return nil, err
		}
		d.reconfigure()
		d.lastDiscoveryUpdate = time.Now()
	}

	services = make([]Service, 0, len(d.servicesMap))
	for _, v := range d.servicesMap {
		services = append(services, v)
	}
	return services, nil
}

// RemoveIfNonRunning remove a service if the service is not running
//
// This is useful to remove persited service that no longer run.
func (d *Discovery) RemoveIfNonRunning(ctx context.Context, services []Service) {
	d.l.Lock()
	defer d.l.Unlock()
	deleted := false
	for _, v := range services {
		key := nameContainer{name: v.Name, containerID: v.ContainerID}
		if _, ok := d.servicesMap[key]; ok {
			deleted = true
		}
		delete(d.servicesMap, key)
	}
	if deleted {
		if _, err := d.discovery(ctx, 0); err != nil {
			logger.V(2).Printf("Error during discovery during RemoveIfNonRunning: %v", err)
		}
	}
}

func (d *Discovery) reconfigure() {
	err := d.configureMetricInputs(d.lastConfigservicesMap, d.servicesMap)
	if err != nil {
		logger.Printf("Unable to update metric inputs: %v", err)
	}
	d.configureChecks(d.lastConfigservicesMap, d.servicesMap)
	d.lastConfigservicesMap = d.servicesMap
}

func (d *Discovery) updateDiscovery(ctx context.Context, maxAge time.Duration) error {
	r, err := d.dynamicDiscovery.Discovery(ctx, maxAge)
	if err != nil {
		return err
	}

	servicesMap := make(map[nameContainer]Service)
	for key, service := range d.servicesMap {
		service.Active = false
		servicesMap[key] = service
	}

	for _, service := range r {
		key := nameContainer{
			name:        service.Name,
			containerID: service.ContainerID,
		}
		if previousService, ok := servicesMap[key]; ok {
			if previousService.hasNetstatInfo && !service.hasNetstatInfo {
				service.ListenAddresses = previousService.ListenAddresses
				service.IPAddress = previousService.IPAddress
				service.hasNetstatInfo = previousService.hasNetstatInfo
			}
		}
		servicesMap[key] = service
	}

	d.servicesMap = servicesMap
	return nil
}
