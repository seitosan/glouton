package check

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"agentgo/types"
)

type accumulator interface {
	AddFieldsWithStatus(measurement string, fields map[string]interface{}, tags map[string]string, statuses map[string]types.StatusDescription, createStatusOf bool, t ...time.Time)
}

// baseCheck perform a service.
//
// The check does:
// * use mainCheck to perform the primary check (protocol specific)
// * open & close a TCP connection on all tcpAddresses (with exclusion of mainTCPAddress if set)
//
// tcpAddresses is supposed to contains mainTCPAddress
//
// If persistentConnection is active, when check successed, this checker will maintain a TCP connection
// to each tcpAddresses to detect service failture quickly.
//
// The check is run at the first of:
// * One minute after last check
// * 30 seconds after checks change to not Ok (to quickly recover from a service restart)
// * (if persistentConnection is active) after a TCP connection is broken
type baseCheck struct {
	metricName     string
	item           string
	mainTCPAddress string
	tcpAddresses   []string
	mainCheck      func(ctx context.Context) types.StatusDescription
	acc            accumulator

	timer    *time.Timer
	dialer   *net.Dialer
	triggerC chan interface{}
	cancel   func()
	wg       sync.WaitGroup

	persistentConnection bool
}

func newBase(mainTCPAddress string, tcpAddresses []string, persistentConnection bool, mainCheck func(context.Context) types.StatusDescription, metricName string, item string, acc accumulator) *baseCheck {
	return &baseCheck{
		metricName:           metricName,
		item:                 item,
		mainTCPAddress:       mainTCPAddress,
		tcpAddresses:         tcpAddresses,
		persistentConnection: persistentConnection,
		mainCheck:            mainCheck,
		acc:                  acc,

		dialer:   &net.Dialer{},
		timer:    time.NewTimer(0),
		triggerC: make(chan interface{}),
	}
}

// Run execute the TCP check
func (bc *baseCheck) Run(ctx context.Context) error {
	// Open connectionS to address
	// when openned, keep checking that port stay open
	// when port goes from open to close, back to step 1
	// If step 1 fail => trigger check
	// trigger check every minutes (or 30 seconds)
	result := types.StatusDescription{
		CurrentStatus:     types.StatusOk,
		StatusDescription: "initial status - description is ignored",
	}
	for {
		select {
		case <-ctx.Done():
			if bc.cancel != nil {
				bc.cancel()
				bc.cancel = nil
			}
			bc.wg.Wait()
			return nil
		case <-bc.triggerC:
			if !bc.timer.Stop() {
				<-bc.timer.C
			}
			result = bc.check(ctx, result)
		case <-bc.timer.C:
			result = bc.check(ctx, result)
		}
	}
}

func (bc *baseCheck) check(ctx context.Context, previousStatus types.StatusDescription) types.StatusDescription {
	// do the check
	// if successful, ensure socket are open
	// if fail, ensure socket are closed
	// if just fail (ok -> critical), do a fast check
	result := bc.doCheck(ctx)
	if ctx.Err() != nil {
		return previousStatus
	}
	timerDone := false
	if result.CurrentStatus != types.StatusOk {
		if bc.cancel != nil {
			bc.cancel()
			bc.wg.Wait()
			bc.cancel = nil
		}
		if previousStatus.CurrentStatus == types.StatusOk {
			bc.timer.Reset(30 * time.Second)
			timerDone = true
		}
	} else {
		bc.openSockets(ctx)
	}

	if !timerDone {
		bc.timer.Reset(time.Minute)
	}
	log.Printf("DBG2: check for %#v on %#v: %v", bc.metricName, bc.item, result)
	labels := make(map[string]string)
	if bc.item != "" {
		labels["item"] = bc.item
	}
	bc.acc.AddFieldsWithStatus(
		"",
		map[string]interface{}{
			bc.metricName: result.CurrentStatus.NagiosCode(),
		},
		labels,
		map[string]types.StatusDescription{bc.metricName: result},
		false,
	)
	return result
}

func (bc *baseCheck) doCheck(ctx context.Context) (result types.StatusDescription) {
	if bc.mainCheck != nil {
		if result = bc.mainCheck(ctx); result.CurrentStatus != types.StatusOk {
			return result
		}
	}
	for _, addr := range bc.tcpAddresses {
		if addr == bc.mainTCPAddress {
			continue
		}
		if subResult := checkTCP(ctx, addr, nil, nil, nil); subResult.CurrentStatus != types.StatusOk {
			return subResult
		} else if !result.CurrentStatus.IsSet() {
			result = subResult
		}
	}
	if !result.CurrentStatus.IsSet() {
		return types.StatusDescription{
			CurrentStatus: types.StatusOk,
		}
	}
	return result
}

func (bc *baseCheck) openSockets(ctx context.Context) {
	if bc.cancel != nil {
		// socket are already open
		return
	}
	ctx2, cancel := context.WithCancel(ctx)
	bc.cancel = cancel

	for _, addr := range bc.tcpAddresses {
		addr := addr
		bc.wg.Add(1)
		go func() {
			defer bc.wg.Done()
			bc.openSocket(ctx2, addr)
		}()
	}
}

func (bc *baseCheck) openSocket(ctx context.Context, addr string) {
	for ctx.Err() == nil {
		longSleep := bc.openSocketOnce(ctx, addr)
		delay := 10 * time.Second
		if !longSleep {
			delay = time.Second
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	}
}

func (bc *baseCheck) openSocketOnce(ctx context.Context, addr string) (longSleep bool) {
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := bc.dialer.DialContext(ctx2, "tcp", addr)
	if err != nil {
		log.Printf("DBG2: fail to open TCP connection to %#v: %v", addr, err)
		select {
		case bc.triggerC <- nil:
		default:
		}
		return true
	}
	defer conn.Close()
	buffer := make([]byte, 4096)
	for ctx.Err() == nil {
		err := conn.SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			log.Printf("DBG2: Unable to SetDeadline() for %#v: %v", addr, err)
			return false
		}
		_, err = conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("DBG2: Unable to Read() from %#v: %v", addr, err)
			return false
		}
	}
	return false
}