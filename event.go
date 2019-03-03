package godim

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

// EventState will give the status of an event throu the application
type EventState int

const (
	// ESEmitted means he is currently being processed amongst switch, interceptor and receiver
	ESEmitted EventState = iota
	// ESResolved means every interceptor and receiver have finished their job on this event
	ESResolved
	// ESAborted can be set by an interceptor, it will stop event processing
	ESAborted
	// ESError means there is 1 or more failure during treatment
	ESError
)

// Event an Event can be transmitted from an Emitter to any Receiver
//
// they're not yet immutable but needs to be considered as they can be concucrrently accessed
//
// The id is set internally and can be override only if another generator is used.
//
// metadatas can be set through the AddMetadata(key string, value interface{}) and retrieve throu GetMetadata(key string)
//
// an event is locked after "interceptor" phase. it means his metadata cannot be modified throu previous methods
//
type Event struct {
	Type     string
	Payload  map[string]interface{}
	metadata eventMetadata
}

type eventMetadata struct {
	id             uint64
	state          EventState
	datas          map[string]interface{}
	lock           bool
	states         map[string]EventState
	mu             *sync.Mutex
	totalListeners int
	finalizer      EventFinalizer
	abortReason    string
}

// GetID Retrieve the id of an event
func (event *Event) GetID() uint64 {
	return event.metadata.id
}

// Abort can be used to abort event dispatching after the current interceptor
//
// a locked event cannot be aborted
func (event *Event) Abort(current EventInterceptor, reason string) {
	if event.metadata.lock {
		return
	}
	event.setState(current.Key(), ESAborted)
	event.metadata.abortReason = reason
}

func (event *Event) setID(id uint64) {
	event.metadata.id = id
}

// AddMetadata add a key value pair of metadata to the event
//
// an error will occur if the event is locked or if the key is already set
func (event *Event) AddMetadata(key string, value interface{}) error {
	if event.metadata.lock {
		return newError(fmt.Errorf("event id %v already lock", event.metadata.id)).SetErrType(ErrTypeEvent)
	}
	if _, ok := event.metadata.datas[key]; ok {
		return newError(fmt.Errorf("unable to set metadata key %s. already setted", key)).SetErrType(ErrTypeEvent)
	}
	event.metadata.datas[key] = value
	return nil
}

// GetMetadata retrieve a metadata
func (event *Event) GetMetadata(key string) interface{} {
	return event.metadata.datas[key]
}

// GetState return the current state
func (event *Event) GetState() EventState {
	return event.metadata.state
}

// GetStates return a copy of current states
//
// can be accessed by multiple go routine
func (e eventMetadata) GetStates() map[string]EventState {
	e.mu.Lock()
	states := make(map[string]EventState)
	for k, v := range e.states {
		states[k] = v
	}
	e.mu.Unlock()
	return states
}

func (event *Event) setStateIf(key string, ifstate, state EventState) {
	e := event.metadata
	e.mu.Lock()
	if e.states[key] == ifstate {
		e.mu.Unlock()
		event.setState(key, state)
	} else {
		e.mu.Unlock()
	}
}

func (event *Event) setState(key string, state EventState) {
	event.metadata.mu.Lock()
	event.metadata.states[key] = state
	nberr := 0
	nbres := 0
	nbemit := 0
	for _, s := range event.metadata.states {
		switch s {
		case ESAborted:
			event.metadata.state = ESAborted
			event.metadata.mu.Unlock()
			event.runFinalizer()
			return
		case ESEmitted:
			nbemit = nbemit + 1
			// we can stop verifying states as we still have something running
			break
		case ESResolved:
			nbres = nbres + 1
		case ESError:
			nberr = nberr + 1
		}
	}
	if nbemit == 0 && event.metadata.totalListeners == nberr+nbres {
		if nberr > 0 {
			event.metadata.state = ESError
		} else {
			event.metadata.state = ESResolved
		}
		event.metadata.mu.Unlock()
		event.runFinalizer()
		return
	}
	event.metadata.mu.Unlock()
}

// Emitter The base interface of an emitter
type Emitter interface {
	prepareRun(chan *Event, IDGenerator, map[string]int, EventFinalizer)
	Emit(*Event)
}

// EventEmitter event emitter
type EventEmitter struct {
	idGenerator    IDGenerator
	eventChan      chan *Event
	listenersCount map[string]int
	finalizer      EventFinalizer
}

func (ee *EventEmitter) prepareRun(e chan *Event, idGen IDGenerator, lc map[string]int, f EventFinalizer) {
	ee.idGenerator = idGen
	ee.eventChan = e
	ee.listenersCount = lc
	ee.finalizer = f
}

// Emit emits an event
func (ee *EventEmitter) Emit(event *Event) {
	event.metadata.id = ee.idGenerator.NextID()
	event.metadata.state = ESEmitted
	event.metadata.mu = &sync.Mutex{}
	event.metadata.datas = make(map[string]interface{})
	event.metadata.totalListeners = ee.listenersCount[event.Type]
	event.metadata.states = make(map[string]EventState, event.metadata.totalListeners)
	event.metadata.finalizer = ee.finalizer
	ee.eventChan <- event
}

// EventReceiver event receiver is like observer,
//
// all receiveEvent are received in a choregraphic pattern, asynchronously
//
// the HandleEventType filter the events this Receiver wants to receive
type EventReceiver interface {
	Identifier
	ReceiveEvent(*Event) error
	HandleEventTypes() []string
}

// EventInterceptor interceptor interface
//
// an Interceptor
//
// - will see all event goes throu intercept method
//
// - can call Abort on an Event
//
// - has a priority : there can't be 2 interceptors at the same priority
type EventInterceptor interface {
	Identifier
	Intercept(*Event) error
	InterceptPriority() int
}

// EventFinalizer event finalizer can be declared
type EventFinalizer interface {
	Finalize(*Event)
}

// IDGenerator handle id generation
type IDGenerator interface {
	SetSeed(uint64) error
	NextID() uint64
}

type defaultIDGenerator struct {
	seed  uint64
	mutex *sync.Mutex
	next  uint64
}

func (dig *defaultIDGenerator) SetSeed(seed uint64) error {
	if dig.seed != 0 {
		return newError(errors.New("default generator seed can be changed only once"))
	}
	dig.seed = seed
	return nil
}

func (dig *defaultIDGenerator) NextID() uint64 {
	dig.mutex.Lock()
	next := dig.next
	dig.next = dig.next + 1
	dig.mutex.Unlock()
	return next
}

func newGenerator() IDGenerator {
	dig := &defaultIDGenerator{
		seed:  0,
		mutex: &sync.Mutex{},
		next:  0,
	}
	return dig
}

// EventSwitch this is not a hub, we want a switch
type EventSwitch struct {
	mainChain        chan *Event
	close            chan struct{}
	emitters         []Emitter
	receivers        map[string][]EventReceiver
	interceptors     map[int]EventInterceptor
	listenerCount    map[string]int
	orderedIntercept []int
	running          bool
	hasInterceptor   bool
	idGenerator      IDGenerator
	eventFinalizer   EventFinalizer
}

// NewEventSwitch build a new event switch
func NewEventSwitch(bufferSize int) *EventSwitch {
	return &EventSwitch{
		mainChain:        make(chan *Event, bufferSize),
		close:            make(chan struct{}),
		receivers:        make(map[string][]EventReceiver),
		interceptors:     make(map[int]EventInterceptor),
		listenerCount:    make(map[string]int),
		orderedIntercept: nil,
		running:          false,
		hasInterceptor:   true,
		idGenerator:      newGenerator(),
		eventFinalizer:   nil,
	}
}

// WithIDGenerator change the id generator used.
//
// if the switch is running, do nothing
func (es *EventSwitch) WithIDGenerator(idg IDGenerator) *EventSwitch {
	if es.running {
		return es
	}
	es.idGenerator = idg
	return es
}

// WithSeed change the seed of the generator
//
// if the switch is running, do nothing
func (es *EventSwitch) WithSeed(seed uint64) *EventSwitch {
	if es.running {
		return es
	}
	err := es.idGenerator.SetSeed(seed)
	if err != nil {
		log.Println("[Godim EventSwitch]Trying to change the seed with a running switch")
	}
	return es
}

// WithEventFinalizer declare an event finalizer that will be called at the end of an event management
func (es *EventSwitch) WithEventFinalizer(f EventFinalizer) *EventSwitch {
	if es.running {
		return es
	}
	es.eventFinalizer = f
	return es
}

// AddEmitter add an emitter
func (es *EventSwitch) AddEmitter(e Emitter) error {
	if es.running {
		return newError(errors.New("Can't add an emitter on the fly yet")).SetErrType(ErrTypeEvent)
	}
	es.emitters = append(es.emitters, e)
	return nil
}

// AddReceiver add a receiver
func (es *EventSwitch) AddReceiver(e EventReceiver) error {
	if es.running {
		return newError(errors.New("Can't add a receiver on the fly yet")).SetErrType(ErrTypeEvent)
	}
	for _, et := range e.HandleEventTypes() {
		es.receivers[et] = append(es.receivers[et], e)
	}
	return nil
}

// AddInterceptor add an interceptor. Interceptor must be prioritized through InterceptPriority(), only one interceptor can run at the same priority
func (es *EventSwitch) AddInterceptor(e EventInterceptor) error {
	if es.running {
		return newError(errors.New("Can't add an interceptor on the fly")).SetErrType(ErrTypeEvent)
	}
	if _, ok := es.interceptors[e.InterceptPriority()]; ok {
		return newError(fmt.Errorf("another interceptor already declared on priority %v", e.InterceptPriority())).SetErrType(ErrTypeEvent)
	}
	es.interceptors[e.InterceptPriority()] = e
	es.hasInterceptor = true
	return nil
}

// Start will start the event switch
func (es *EventSwitch) Start() {
	if es.running {
		return
	}
	nbInterceptor := 0
	if es.hasInterceptor {
		es.orderedIntercept = make([]int, len(es.interceptors))
		for prio := range es.interceptors {
			es.orderedIntercept[nbInterceptor] = prio
			nbInterceptor++
		}
	}
	for k, v := range es.receivers {
		es.listenerCount[k] = nbInterceptor + len(v)
	}
	for _, e := range es.emitters {
		e.prepareRun(es.mainChain, es.idGenerator, es.listenerCount, es.eventFinalizer)
	}
	es.running = true
	go es.run()
}

// Stop will stop the event switch.
func (es *EventSwitch) Stop() {
	if !es.running {
		return
	}
	es.close <- struct{}{}
	es.running = false
}

// Close close all chan
func (es *EventSwitch) Close() {
	es.Stop()
	close(es.close)
	close(es.mainChain)
}

func (es *EventSwitch) run() {
	for {
		select {
		case event, valid := <-es.mainChain:
			if !es.running {
				break
			}
			if !valid {
				break
			}
			go es.switchEvent(event)
		case <-es.close:
			return
		}
	}
}

func (es *EventSwitch) switchEvent(event *Event) {
	if es.hasInterceptor {
		for _, prio := range es.orderedIntercept {
			interceptor := es.interceptors[prio]
			es.runInterceptorWithRecover(interceptor, event)
			if event.metadata.state == ESAborted {
				log.Printf("[Godim EventSwitch] aborting event id [%v]\n", event.metadata.id)
				return
			}
		}
	}
	event.metadata.lock = true
	rs, ok := es.receivers[event.Type]
	if ok {
		for _, receiver := range rs {
			go es.runObserverWithRecover(receiver, event)
		}
	}
}

func (es *EventSwitch) runObserverWithRecover(receiver EventReceiver, event *Event) {
	event.setState(receiver.Key(), ESEmitted)
	defer internalRecover(event, receiver)
	err := receiver.ReceiveEvent(event)
	if err != nil {
		event.setState(receiver.Key(), ESError)
	}
}

func (es *EventSwitch) runInterceptorWithRecover(interceptor EventInterceptor, event *Event) {
	event.setState(interceptor.Key(), ESEmitted)
	defer internalRecover(event, interceptor)
	err := interceptor.Intercept(event)
	if err != nil {
		event.setState(interceptor.Key(), ESError)
	}
}

func internalRecover(event *Event, identifier Identifier) {
	if rec := recover(); rec != nil {
		event.setState(identifier.Key(), ESError)
		dumpRec(rec, "godim: panic receiving event")
	} else {
		event.setStateIf(identifier.Key(), ESEmitted, ESResolved)
	}
}

func dumpRec(rec interface{}, msg string) {
	// const size = 64 << 10
	// buffer := make([]byte, size)
	// buffer = buffer[:runtime.Stack(buffer, false)]
	// log.Printf("%s: %v\n%s", msg, rec, buffer)
	log.Printf("%s: %v\n", msg, rec)
}

func (event *Event) runFinalizer() {
	if event.metadata.finalizer != nil {
		defer func() {
			if rec := recover(); rec != nil {
				dumpRec(rec, "godim: panic finalizing event")
			}
		}()
		event.metadata.finalizer.Finalize(event)
	}
}
