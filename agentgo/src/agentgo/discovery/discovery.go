package discovery

import (
	"agentgo/types"
	"context"
	"log"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
)

// Discovery implement the full discovery mecanisme. It will take informations
// from both the dynamic discovery (service currently running) and previously
// detected services.
// It will configure metrics input and add them to a Collector
type Discovery struct {
	l sync.Mutex

	dynamicDiscovery Discoverer

	servicesMap         map[nameContainer]Service
	lastDiscoveryUpdate time.Time

	activeInput           map[nameContainer]int
	activeCheckCancelFunc map[nameContainer]func()
	coll                  Collector
	acc                   Accumulator
}

// Collector will gather metrics for added inputs
type Collector interface {
	AddInput(input telegraf.Input, shortName string) int
	RemoveInput(int)
}

// Accumulator will gather metrics point for added checks
type Accumulator interface {
	AddFieldsWithStatus(measurement string, fields map[string]interface{}, tags map[string]string, statuses map[string]types.StatusDescription, createStatusOf bool, t ...time.Time)
}

// New returns a new Discovery
func New(dynamicDiscovery Discoverer, coll Collector, initialServices []Service, acc Accumulator) *Discovery {
	servicesMap := make(map[nameContainer]Service, len(initialServices))
	for _, v := range initialServices {
		key := nameContainer{
			name:        v.Name,
			containerID: v.ContainerID,
		}
		servicesMap[key] = v
	}
	return &Discovery{
		dynamicDiscovery:      dynamicDiscovery,
		servicesMap:           servicesMap,
		coll:                  coll,
		acc:                   acc,
		activeInput:           make(map[nameContainer]int),
		activeCheckCancelFunc: make(map[nameContainer]func()),
	}
}

// Close stop & cleanup inputs & check created by the discovery
func (d *Discovery) Close() {
	d.l.Lock()
	defer d.l.Unlock()
	_ = d.configureMetricInputs(d.servicesMap, nil)
	_ = d.configureChecks(d.servicesMap, nil)
}

// Discovery detect service on the system and return a list of Service object.
//
// It may trigger an update of metric inputs present in the Collector
func (d *Discovery) Discovery(ctx context.Context, maxAge time.Duration) (services []Service, err error) {
	d.l.Lock()
	defer d.l.Unlock()

	if time.Since(d.lastDiscoveryUpdate) > maxAge {
		d.l.Unlock()
		servicesMap, err := d.updateDiscovery(ctx, maxAge)
		d.l.Lock()
		if err != nil {
			return nil, err
		}
		err = d.configureMetricInputs(d.servicesMap, servicesMap)
		if err != nil {
			log.Printf("Unable to update metric inputs: %v", err)
		}
		err = d.configureChecks(d.servicesMap, servicesMap)
		if err != nil {
			log.Printf("Unable to update metric checks: %v", err)
		}
		d.servicesMap = servicesMap
		d.lastDiscoveryUpdate = time.Now()
	}

	services = make([]Service, 0, len(d.servicesMap))
	for _, v := range d.servicesMap {
		services = append(services, v)
	}
	return services, nil
}

func (d *Discovery) updateDiscovery(ctx context.Context, maxAge time.Duration) (map[nameContainer]Service, error) {
	r, err := d.dynamicDiscovery.Discovery(ctx, maxAge)
	if err != nil {
		return nil, err
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

	return servicesMap, nil
}
