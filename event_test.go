package godim

import (
	"log"
	"strconv"
	"sync"
	"testing"
	"time"
)

const (
	EventTypeA = "a"
	EventTypeB = "b"

	ThrowPanicReceiverKey = "ThrowPanicReceiver"
)

var AllEventTypes = []string{EventTypeA, EventTypeB}

func TestEventSwitch_Start_shouldHandleIncomingEvents(t *testing.T) {
	eventSwitch := NewEventSwitch(10)

	emitter := new(SimpleEmitter)
	eventSwitch.AddEmitter(emitter)

	receiver := newCollectEventIdReceiver()
	eventSwitch.AddReceiver(receiver)

	eventSwitch.Start()

	for i := 0; i < 100; i++ {
		emitter.Emit(newEmptyEvent(EventTypeA))
		time.Sleep(1 * time.Millisecond)
	}
	time.Sleep(5 * time.Millisecond)

	if receiver.nbReceived != 100 {
		t.Fatal("Expecting 100 events but received: ", receiver.nbReceived, ".")
	}

	if numberOfId := len(receiver.ids); numberOfId != 100 {
		t.Fatal("Expecting 100 IDs but collected: ", numberOfId, ".")
	}

	eventSwitch.Close()

	if eventSwitch.running {
		t.Fatal("EventSwitch is still running despite Close method has been called.")
	}
}

func TestEventSwitch_CloseGracefully_shouldWaitForAllEventsToBeProcessed(t *testing.T) {
	eventSwitch := NewEventSwitch(10)

	emitter := new(SimpleEmitter)
	eventSwitch.AddEmitter(emitter)

	delayInterceptor := new(DelayInterceptor)
	eventSwitch.AddInterceptor(delayInterceptor)

	receiver := newCollectEventIdReceiver()
	eventSwitch.AddReceiver(receiver)

	eventSwitch.Start()

	for i := 0; i < 100; i++ {
		emitter.Emit(newEmptyEvent(EventTypeA))
	}

	eventSwitch.CloseGracefully()

	if receiver.nbReceived != 100 {
		t.Fatal("Expecting 100 events but received: ", receiver.nbReceived, ".")
	}

	if numberOfId := len(receiver.ids); numberOfId != 100 {
		t.Fatal("Expecting 100 IDs but collected: ", numberOfId, ".")
	}

	if eventSwitch.running {
		t.Fatal("EventSwitch is still running despite CloseGracefully method has been called.")
	}
}

func TestEventSwitch_Close_shouldNotWaitForUnprocessedEvents(t *testing.T) {
	eventSwitch := NewEventSwitch(10)

	emitter := new(SimpleEmitter)
	eventSwitch.AddEmitter(emitter)

	delayInterceptor := new(DelayInterceptor)
	eventSwitch.AddInterceptor(delayInterceptor)

	receiver := newCollectEventIdReceiver()
	eventSwitch.AddReceiver(receiver)

	eventSwitch.Start()

	for i := 0; i < 100; i++ {
		emitter.Emit(newEmptyEvent(EventTypeA))
	}

	eventSwitch.Close()

	if receiver.nbReceived == 100 {
		t.Fatal("Expecting less than 100 events.")
	}

	if numberOfId := len(receiver.ids); numberOfId == 100 {
		t.Fatal("Expecting less than 100 IDs.")
	}

	if eventSwitch.running {
		t.Fatal("EventSwitch is still running despite Close method has been called.")
	}
}

func TestEventSwitch_Start_shouldHandlePanicGently(t *testing.T) {
	eventSwitch := NewEventSwitch(10)

	emitter := new(SimpleEmitter)
	eventSwitch.AddEmitter(emitter)

	testReceiver := newCollectEventIdReceiver()
	eventSwitch.AddReceiver(testReceiver)

	throwPanicReceiver := new(ThrowPanicReceiver)
	eventSwitch.AddReceiver(throwPanicReceiver)

	eventSwitch.Start()

	eventOne := newEmptyEvent(EventTypeA)
	emitter.Emit(eventOne)
	time.Sleep(2 * time.Millisecond)

	eventTwo := newEmptyEvent(EventTypeB)
	emitter.Emit(eventTwo)
	time.Sleep(2 * time.Millisecond)

	if testReceiver.nbReceived != 2 {
		t.Fatal("Event not received during a riot.")
	}

	if eventOneState := eventOne.metadata.states[ThrowPanicReceiverKey]; eventOneState != ESError {
		t.Fatalf("EventOne's state is not correct: %d.", eventOneState)
	}

	if eventTwoState := eventTwo.metadata.states[ThrowPanicReceiverKey]; eventTwoState != ESError {
		t.Fatalf("EventTwo's state is not correct: %d.", eventTwoState)
	}
}

func TestEventSwitch_Start_shouldHandleMassiveAmountOfEvents(t *testing.T) {
	numberOfProducers := 50
	numberOfReceivers := 10

	producers := make(map[int]*CounterProducer, numberOfProducers)
	receivers := make(map[int]*CounterReceiver, numberOfReceivers)

	finalizer := &CounterFinalizer{mu: &sync.Mutex{}}

	eventSwitch := NewEventSwitch(10).WithEventFinalizer(finalizer)

	for i := 0; i < numberOfProducers; i++ {
		producer := newCounterProducer()
		eventSwitch.AddEmitter(producer)
		producers[i] = producer
	}

	for i := 0; i < numberOfReceivers; i++ {
		receiver := newCounterReceiver(i)
		receivers[i] = receiver
		eventSwitch.AddReceiver(receiver)
	}

	interceptor := newCounterInterceptor()
	eventSwitch.AddInterceptor(interceptor)

	eventSwitch.Start()

	for i := 0; i < numberOfProducers; i++ {
		go producers[i].EmitWhile()
	}
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < numberOfProducers; i++ {
		producers[i].closed <- struct{}{}
	}
	time.Sleep(1 * time.Millisecond)

	eventSwitch.CloseGracefully()

	totalNumberOfEmittedEvents := 0
	for i := 0; i < numberOfProducers; i++ {
		totalNumberOfEmittedEvents += producers[i].emittedEventCount
	}

	for i := 1; i < numberOfReceivers; i++ {
		previousReceiverCount := receivers[i-1].receivedEventCount
		currentReceiverCount := receivers[i].receivedEventCount
		if previousReceiverCount != currentReceiverCount {
			t.Fatalf("All receivers should receive the same number of events. Receiver #%d received %d but previous receiver got %d.",
				i, currentReceiverCount, previousReceiverCount)
		}
	}
	totalNumberOfReceivedEvents := receivers[0].receivedEventCount

	if finalizer.finalizedEventCount != totalNumberOfEmittedEvents {
		t.Fatalf("Finalizer did not processed all events. Emitted %d but finalized %d.", totalNumberOfEmittedEvents, finalizer.finalizedEventCount)
	}

	if interceptor.interceptedEventCount != totalNumberOfEmittedEvents {
		t.Fatalf("Should have intercepted %d but got %d.", totalNumberOfEmittedEvents, totalNumberOfReceivedEvents)
	}

	if totalNumberOfReceivedEvents == totalNumberOfEmittedEvents {
		t.Fatal("Should have some aborted events.")
	}

	receivedAndAbortedEventCount := totalNumberOfReceivedEvents + interceptor.abortedEventCount
	if receivedAndAbortedEventCount != totalNumberOfEmittedEvents {
		t.Fatalf("The total number of event should equal the number of received events plus the number of aborted events. Got %d but expected %d.",
			receivedAndAbortedEventCount, totalNumberOfEmittedEvents)
	}
}

type DelayInterceptor struct{}

func (i *DelayInterceptor) Key() string {
	return "DelayInterceptor"
}

func (i *DelayInterceptor) InterceptPriority() int {
	return -1
}

func (i *DelayInterceptor) Intercept(e *Event) error {
	time.Sleep(1 * time.Second)
	return nil
}

type ThrowPanicReceiver struct {
}

func (tpr *ThrowPanicReceiver) Key() string {
	return ThrowPanicReceiverKey
}

func (tpr *ThrowPanicReceiver) HandleEventTypes() []string {
	return AllEventTypes
}

func (tpr *ThrowPanicReceiver) ReceiveEvent(e *Event) error {
	panic("this is my role")
}

type SimpleEmitter struct {
	EventEmitter
}

type CollectEventIdReceiver struct {
	nbReceived int
	ids        map[uint64]bool
	mu         *sync.Mutex
}

func newCollectEventIdReceiver() *CollectEventIdReceiver {
	receiver := new(CollectEventIdReceiver)
	receiver.ids = make(map[uint64]bool)
	receiver.mu = &sync.Mutex{}
	return receiver
}

func (tr *CollectEventIdReceiver) Key() string {
	return "CollectEventIdReceiver"
}

func (tr *CollectEventIdReceiver) HandleEventTypes() []string {
	return AllEventTypes
}

func (tr *CollectEventIdReceiver) ReceiveEvent(e *Event) error {
	tr.mu.Lock()
	tr.ids[e.metadata.id] = true
	tr.mu.Unlock()
	tr.nbReceived += 1
	return nil
}

func newEmptyEvent(eventType string) *Event {
	return &Event{
		Type: eventType,
	}
}

func newEventA(payload map[string]interface{}) *Event {
	return &Event{
		Type:    EventTypeA,
		Payload: payload,
	}
}

type CounterProducer struct {
	EventEmitter
	emittedEventCount int
	closed            chan struct{}
}

func newCounterProducer() *CounterProducer {
	return &CounterProducer{
		closed: make(chan struct{}),
	}
}

func (p *CounterProducer) EmitWhile() {
	for {
		p.emittedEventCount = p.emittedEventCount + 1

		payload := make(map[string]interface{})
		payload["finalizedEventCount"] = p.emittedEventCount

		p.Emit(newEventA(payload))
		time.Sleep(3 * time.Millisecond)

		var ok bool

		select {
		case <-p.closed:
			ok = true
		default:
			ok = false
		}

		if ok {
			break
		}
	}
}

type CounterReceiver struct {
	mu                 *sync.Mutex
	index              int
	receivedEventCount int
}

func newCounterReceiver(index int) *CounterReceiver {
	return &CounterReceiver{
		index: index,
		mu:    &sync.Mutex{},
	}
}

func (gr *CounterReceiver) Key() string {
	return "CounterReceiver-" + strconv.Itoa(gr.index)
}

func (gr *CounterReceiver) HandleEventTypes() []string {
	return AllEventTypes
}

func (gr *CounterReceiver) ReceiveEvent(e *Event) error {
	gr.mu.Lock()
	if e.GetID()%10 == 0 {
		log.Println("should not have received this event")
	}
	gr.receivedEventCount += 1
	gr.mu.Unlock()
	return nil
}

type CounterEventInterceptor struct {
	interceptedEventCount int
	mu                    *sync.Mutex
	abortedEventCount     int
}

func newCounterInterceptor() *CounterEventInterceptor {
	return &CounterEventInterceptor{
		mu: &sync.Mutex{},
	}
}

func (i *CounterEventInterceptor) Key() string {
	return "CounterEventInterceptor"
}

func (i *CounterEventInterceptor) InterceptPriority() int {
	return -1
}

func (i *CounterEventInterceptor) Intercept(e *Event) error {
	i.mu.Lock()
	i.interceptedEventCount += 1

	if e.GetID()%10 == 0 {
		e.Abort(i, "abort every 10")
		i.abortedEventCount += 1
	}
	i.mu.Unlock()
	return nil
}

type CounterFinalizer struct {
	finalizedEventCount int
	mu                  *sync.Mutex
}

func (f *CounterFinalizer) Finalize(e *Event) {
	f.mu.Lock()
	f.finalizedEventCount = f.finalizedEventCount + 1
	f.mu.Unlock()
}
