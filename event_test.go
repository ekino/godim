package godim

import (
	"fmt"
	"log"
	"testing"
	"time"
)

type Testeur struct {
	EventEmitter
}

func TestEmitter(t *testing.T) {
	tester := new(Testeur)
	myChan := make(chan *Event, 100)
	tester.eventChan = myChan
	tester2 := new(Testeur)
	tester2.eventChan = myChan

	results := make(map[string]*Event)

	go func() {
		for {
			select {
			case event := <-myChan:
				go managerEvent(results, event)
			}
		}
	}()
	for i := 0; i < 10; i++ {
		e := &Event{
			Type: fmt.Sprintln("my ", i),
		}
		tester.Emit(e)
		time.Sleep(2 * time.Millisecond)
		e2 := &Event{
			Type: fmt.Sprintln("sec ", i),
		}
		tester2.Emit(e2)
		time.Sleep(2 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)
	if len(results) != 20 {
		t.Fatal("not the right number of events : ", len(results))
	}
}

func managerEvent(results map[string]*Event, e *Event) {
	log.Println("Event received : ", e.Type)
	results[e.Type] = e

}

type Rec1 struct {
	nbReceived int
}

func (rec1 *Rec1) HandleEventTypes() []string {
	return []string{
		"a",
		"b",
	}
}

func (rec1 *Rec1) ReceiveEvent(e *Event) {
	rec1.nbReceived = rec1.nbReceived + 1
}

func TestSwitch(t *testing.T) {
	es := NewEventSwitch(10)
	t1 := new(Testeur)
	es.AddEmitter(t1)
	r1 := new(Rec1)
	es.AddReceiver(r1)

	es.Start()

	for i := 0; i < 100; i++ {
		e := &Event{
			Type: "a",
		}
		t1.Emit(e)
		time.Sleep(1 * time.Millisecond)
	}
	time.Sleep(5 * time.Millisecond)
	if r1.nbReceived != 100 {
		t.Fatal("Received events : ", r1.nbReceived)
	}
	es.Close()

	if es.running {
		t.Fatal("switch still running")
	}
}
