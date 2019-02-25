package godim

import (
	"errors"
	"log"
)

// Event an Event can be transmitted from an Emitter to any Receiver
//
// they're not yet immutable but needs to be considered as they can be concucrrently accessed
type Event struct {
	Type    string
	Payload map[string]interface{}
}

// Emitter The base interface of an emitter
type Emitter interface {
	InitChan(chan *Event)
	Emit(*Event)
}

// EventEmitter event emitter
type EventEmitter struct {
	eventChan chan *Event
}

// InitChan initialize event channel
func (ee *EventEmitter) InitChan(e chan *Event) {
	ee.eventChan = e
}

// Emit emits an event
func (ee *EventEmitter) Emit(event *Event) {
	ee.eventChan <- event
}

// EventReceiver event receiver
type EventReceiver interface {
	ReceiveEvent(*Event)
	HandleEventTypes() []string
}

// EventSwitch this is not a hub, we want a switch
type EventSwitch struct {
	mainChain chan *Event
	close     chan struct{}
	receivers map[string][]EventReceiver
	running   bool
}

// NewEventSwitch build a new event switch
func NewEventSwitch(bufferSize int) *EventSwitch {
	return &EventSwitch{
		mainChain: make(chan *Event, bufferSize),
		close:     make(chan struct{}),
		receivers: make(map[string][]EventReceiver),
		running:   false,
	}
}

// AddEmitter add an emitter
func (es *EventSwitch) AddEmitter(e Emitter) error {
	if es.running {
		return errors.New("Can't add an emitter on the fly yet")
	}
	e.InitChan(es.mainChain)
	return nil
}

// AddReceiver add a receiver
func (es *EventSwitch) AddReceiver(e EventReceiver) error {
	if es.running {
		return errors.New("Can't add a receiver on the fly yet")
	}
	for _, et := range e.HandleEventTypes() {
		es.receivers[et] = append(es.receivers[et], e)
	}
	return nil
}

// Start will start the event switch
func (es *EventSwitch) Start() {
	if es.running {
		return
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
		case event := <-es.mainChain:
			if !es.running {
				break
			}
			go es.switchEvent(event)
			continue
		case <-es.close:
			return
		}
	}
}

func (es *EventSwitch) switchEvent(event *Event) {
	rs, ok := es.receivers[event.Type]
	if ok {
		for _, receiver := range rs {
			go receiver.ReceiveEvent(event)
		}
	} else {
		log.Println("an event is declared with no subscribers :", event.Type)
	}
}
