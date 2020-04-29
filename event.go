package godim

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type EventState int

const (
	// ESEmitted the event is being processed
	ESEmitted EventState = iota
	// ESResolved the event has been successfully processed by the interceptors and the receivers
	ESResolved
	// ESAborted the event processing is stopped which means that it won't be transmitted to any receiver
	ESAborted
	// ESError the event encountered an error during its processing
	ESError
)

// Event an Event can be transmitted from an Emitter to any Receiver.
// It's not yet immutable but needs to be considered as they can be concurrently accessed.
// The id is set internally and can be overridden only if another generator is used.
// The metadata can be set with AddMetadata(key string, value interface{}) and can be retrieved with GetMetadata(key string).
// An event is locked after "interceptor" phase which means its metadata cannot be modified through previous methods.
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
	locked         bool
	states         map[string]EventState
	mu             *sync.Mutex
	totalListeners int
	finalizer      EventFinalizer
	abortReason    string
}

func (event *Event) GetID() uint64 {
	return event.metadata.id
}

// Abort stop the processing of the event.
// This method could be called from any interceptor.
// A locked event cannot be aborted.
func (event *Event) Abort(current EventInterceptor, reason string) {
	if event.metadata.locked {
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
// An error will occur if the event is locked or if the key exists already.
func (event *Event) AddMetadata(key string, value interface{}) error {
	if event.metadata.locked {
		return newError(fmt.Errorf("event id %v already locked", event.metadata.id)).SetErrType(ErrTypeEvent)
	}
	if _, ok := event.metadata.datas[key]; ok {
		return newError(fmt.Errorf("the key '%s' already exists", key)).SetErrType(ErrTypeEvent)
	}
	event.metadata.datas[key] = value
	return nil
}

func (event *Event) GetMetadata(key string) interface{} {
	return event.metadata.datas[key]
}

func (event *Event) GetState() EventState {
	return event.metadata.state
}

// GetStates return a copy of the current states.
//
// This method is thread-safe.
func (e eventMetadata) GetStates() map[string]EventState {
	e.mu.Lock()
	states := make(map[string]EventState)
	for k, v := range e.states {
		states[k] = v
	}
	e.mu.Unlock()
	return states
}

func (event *Event) setStateIf(key string, expectedState, newState EventState) {
	event.metadata.mu.Lock()
	if event.metadata.states[key] == expectedState {
		event.metadata.mu.Unlock()
		event.setState(key, newState)
	} else {
		event.metadata.mu.Unlock()
	}
}

func (event *Event) setState(key string, newState EventState) {
	event.metadata.mu.Lock()

	if newState == ESAborted {
		event.metadata.state = ESAborted
		event.metadata.mu.Unlock()
		event.runFinalizer()
		return
	}

	event.metadata.states[key] = newState

	lessStateThanExpected := len(event.metadata.states) < event.metadata.totalListeners
	notYetResolved := newState == ESEmitted
	if lessStateThanExpected || notYetResolved {
		event.metadata.mu.Unlock()
		return
	}

	resolvedCount, errorCount := countResolvedAndErrorStates(event)
	if event.metadata.totalListeners == errorCount+resolvedCount {
		if errorCount > 0 {
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

func countResolvedAndErrorStates(event *Event) (resolvedCount, errorCount int) {
	for _, s := range event.metadata.states {
		switch s {
		case ESResolved:
			resolvedCount += 1
		case ESError:
			errorCount += 1
		}
	}
	return
}

type Emitter interface {
	prepareRun(chan *Event, IDGenerator, map[string]int, EventFinalizer)
	Emit(*Event)
}

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

// Emit adds metadata to the event then emit it through the channel
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

// EventReceiver
// All events are treated in a choreographic pattern, asynchronously.
// The HandleEventTypes should return the types of event the receiver wants to handle.
type EventReceiver interface {
	Identifier
	ReceiveEvent(*Event) error
	HandleEventTypes() []string
}

// EventInterceptor any events handled by the EventSwitch goes through the interceptors.
// An interceptor can abort an Event to stop its processing beyond the interceptor itself.
// An interceptor must define a priority and 2 interceptors cannot have the same priority.
type EventInterceptor interface {
	Identifier
	Intercept(*Event) error
	InterceptPriority() int
}

type EventFinalizer interface {
	Finalize(*Event)
}

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
	mainChan            chan *Event
	close               chan struct{}
	emitters            []Emitter
	receivers           map[string][]EventReceiver
	interceptors        map[int]EventInterceptor
	listenerCount       map[string]int
	orderedIntercept    []int
	running             bool
	hasInterceptor      bool
	idGenerator         IDGenerator
	eventFinalizer      EventFinalizer
	pendingEventCounter Counter
}

// NewEventSwitch build a new event switch
func NewEventSwitch(bufferSize int) *EventSwitch {
	return &EventSwitch{
		mainChan:            make(chan *Event, bufferSize),
		close:               make(chan struct{}),
		receivers:           make(map[string][]EventReceiver),
		interceptors:        make(map[int]EventInterceptor),
		listenerCount:       make(map[string]int),
		orderedIntercept:    nil,
		running:             false,
		hasInterceptor:      true,
		idGenerator:         newGenerator(),
		eventFinalizer:      nil,
		pendingEventCounter: newCounter(),
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
		return newError(errors.New("can't add an emitter on the fly yet")).SetErrType(ErrTypeEvent)
	}
	es.emitters = append(es.emitters, e)
	return nil
}

// AddReceiver add a receiver
func (es *EventSwitch) AddReceiver(e EventReceiver) error {
	if es.running {
		return newError(errors.New("can't add a receiver on the fly yet")).SetErrType(ErrTypeEvent)
	}
	for _, et := range e.HandleEventTypes() {
		es.receivers[et] = append(es.receivers[et], e)
	}
	return nil
}

// AddInterceptor add an interceptor.
// An error is returned when an interceptor with the same priority is already declared.
func (es *EventSwitch) AddInterceptor(e EventInterceptor) error {
	if es.running {
		return newError(errors.New("can't add an interceptor on the fly")).SetErrType(ErrTypeEvent)
	}
	if _, ok := es.interceptors[e.InterceptPriority()]; ok {
		return newError(fmt.Errorf("another interceptor already declared on priority %v", e.InterceptPriority())).SetErrType(ErrTypeEvent)
	}
	es.interceptors[e.InterceptPriority()] = e
	es.hasInterceptor = true
	return nil
}

// Start initialize the EventSwitch and start it
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
		e.prepareRun(es.mainChan, es.idGenerator, es.listenerCount, es.eventFinalizer)
	}
	es.running = true
	go es.run()
}

func (es *EventSwitch) Stop() {
	if es.running {
		es.close <- struct{}{}
		es.running = false
	}
}

// Close stop the EventEmitter then closes it channels
func (es *EventSwitch) Close() {
	es.Stop()
	close(es.close)
	close(es.mainChan)
}

// CloseGracefully wait for all events to be processed then close the EventSwitch
// This method is blocking until the EventSwitch is stopped.
func (es *EventSwitch) CloseGracefully() {
	if es.running {
		go es.closeOnceAllEventsAreHandled()
		for es.running {
			// Should stop running once all events are processed
		}
	}
}

func (es *EventSwitch) closeOnceAllEventsAreHandled() {
	for es.hasUnreadEvent() || es.hasPendingEvent() {
		time.Sleep(500 * time.Millisecond)
	}

	es.Close()
}

func (es *EventSwitch) hasUnreadEvent() bool {
	return len(es.mainChan) != 0
}

func (es *EventSwitch) hasPendingEvent() bool {
	return es.pendingEventCounter.count != 0
}

func (es *EventSwitch) run() {
	for {
		select {
		case event, valid := <-es.mainChan:
			if es.running && valid {
				go es.switchEvent(event)
			}
		case <-es.close:
			return
		}
	}
}

func (es *EventSwitch) switchEvent(event *Event) {
	es.pendingEventCounter.increase()
	defer es.pendingEventCounter.decrease()

	aborted := es.runInterceptorsIfAny(event)
	if aborted {
		return
	}

	event.metadata.locked = true
	es.runReceivers(event)
}

func (es *EventSwitch) runInterceptorsIfAny(event *Event) bool {
	if es.hasInterceptor {
		for _, priority := range es.orderedIntercept {
			interceptor := es.interceptors[priority]
			es.runInterceptorWithRecover(interceptor, event)

			if event.metadata.state == ESAborted {
				log.Printf("[Godim EventSwitch] aborting event id [%v]\n", event.metadata.id)
				return true
			}
		}
	}
	return false
}

func (es *EventSwitch) runInterceptorWithRecover(interceptor EventInterceptor, event *Event) {
	event.setState(interceptor.Key(), ESEmitted)

	defer internalRecover(event, interceptor)

	err := interceptor.Intercept(event)
	if err != nil {
		event.setState(interceptor.Key(), ESError)
	}
}

func (es *EventSwitch) runReceivers(event *Event) {
	rs, ok := es.receivers[event.Type]
	if ok {
		for _, receiver := range rs {
			go es.runReceiverWithRecover(receiver, event)
		}
	}
}

func (es *EventSwitch) runReceiverWithRecover(receiver EventReceiver, event *Event) {
	event.setState(receiver.Key(), ESEmitted)

	defer internalRecover(event, receiver)

	err := receiver.ReceiveEvent(event)
	if err != nil {
		event.setState(receiver.Key(), ESError)
	}
}

func internalRecover(event *Event, identifier Identifier) {
	if rec := recover(); rec != nil {
		event.setState(identifier.Key(), ESError)
		logCaughtPanic(rec, "[Godim EventSwitch] panic receiving event")
	} else {
		event.setStateIf(identifier.Key(), ESEmitted, ESResolved)
	}
}

func logCaughtPanic(rec interface{}, msg string) {
	log.Printf("%s: %v\n", msg, rec)
}

func (event *Event) runFinalizer() {
	if event.metadata.finalizer != nil {
		defer func() {
			if rec := recover(); rec != nil {
				logCaughtPanic(rec, "[Godim EventSwitch] panic finalizing event")
			}
		}()
		event.metadata.finalizer.Finalize(event)
	}
}

type Counter struct {
	count uint
	mutex *sync.Mutex
}

func newCounter() Counter {
	return Counter{
		mutex: new(sync.Mutex),
	}
}

func (ec *Counter) increase() {
	ec.mutex.Lock()
	ec.count += 1
	ec.mutex.Unlock()
}

func (ec *Counter) decrease() {
	ec.mutex.Lock()
	ec.count -= 1
	ec.mutex.Unlock()
}
