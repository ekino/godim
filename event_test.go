package godim

import (
	"log"
	"sync"
	"testing"
	"time"
)

type Testeur struct {
	EventEmitter
}

type Rec1 struct {
	nbReceived int
	ids        map[uint64]bool
	mu         *sync.Mutex
}

func (rec1 *Rec1) Key() string {
	return "Rec1"
}

func (rec1 *Rec1) HandleEventTypes() []string {
	return []string{
		"a",
		"b",
	}
}

func (rec1 *Rec1) ReceiveEvent(e *Event) error {
	rec1.mu.Lock()
	rec1.ids[e.metadata.id] = true
	rec1.mu.Unlock()
	rec1.nbReceived = rec1.nbReceived + 1
	return nil
}

func TestSwitch(t *testing.T) {
	es := NewEventSwitch(10)
	t1 := new(Testeur)
	es.AddEmitter(t1)
	r1 := new(Rec1)
	r1.ids = make(map[uint64]bool)
	r1.mu = &sync.Mutex{}
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
	if len(r1.ids) != 100 {
		t.Fatal("received ids ", len(r1.ids))
	}
	es.Close()

	if es.running {
		t.Fatal("switch still running")
	}
}

type Panicker struct {
}

func (rec1 *Panicker) Key() string {
	return "Panicker"
}

func (rec1 *Panicker) HandleEventTypes() []string {
	return []string{
		"a",
		"b",
	}
}

func (rec1 *Panicker) ReceiveEvent(e *Event) error {
	panic("this is my role")

}

func TestPanic(t *testing.T) {
	es := NewEventSwitch(10)
	t1 := new(Testeur)
	es.AddEmitter(t1)
	r1 := new(Rec1)
	r1.ids = make(map[uint64]bool)
	r1.mu = &sync.Mutex{}
	es.AddReceiver(r1)
	r2 := new(Panicker)
	es.AddReceiver(r2)

	es.Start()
	e := &Event{
		Type: "a",
	}
	t1.Emit(e)
	time.Sleep(2 * time.Millisecond)
	if r1.nbReceived != 1 {
		t.Fatal("event not received during a riot")
	}
	f := &Event{
		Type: "b",
	}
	t1.Emit(f)
	time.Sleep(2 * time.Millisecond)
	if r1.nbReceived != 2 {
		t.Fatal("event not received during a riot")
	}
	if f.metadata.states["Panicker"] != ESError {
		t.Fatal("state not correct", f.metadata.states["Panicker"])
	}
}

type Producer struct {
	EventEmitter
	nbEmitted int
	typ       string
	closed    chan struct{}
}

func (p *Producer) EmitWhile() {
	for {
		payload := make(map[string]interface{})
		p.nbEmitted = p.nbEmitted + 1
		payload["nb"] = p.nbEmitted
		e := &Event{
			Type:    p.typ,
			Payload: payload,
		}
		p.Emit(e)
		time.Sleep(3 * time.Millisecond)
		var ok bool
		select {
		case <-p.closed:
			ok = true
			// log.Println("closing producer")
		default:
			ok = false
		}
		if ok {
			break
		}
	}
}

type GlobalRec struct {
	mu       *sync.Mutex
	received int
	typ      string
}

func (gr *GlobalRec) Key() string {
	return "GR"
}

func (gr *GlobalRec) HandleEventTypes() []string {
	return []string{
		gr.typ,
	}
}

func (gr *GlobalRec) ReceiveEvent(e *Event) error {
	gr.mu.Lock()
	if e.GetID()%10 == 0 {
		log.Println("should not have received this event")
	}
	gr.received = gr.received + 1
	gr.mu.Unlock()
	return nil
}

type Inter struct {
	received int
	mu       *sync.Mutex
	aborted  int
}

func (i *Inter) Key() string {
	return "inter"
}
func (i *Inter) InterceptPriority() int {
	return -1
}
func (i *Inter) Intercept(e *Event) error {
	i.mu.Lock()
	i.received = i.received + 1

	if e.GetID()%10 == 0 {
		e.Abort(i, "abort every 10")
		i.aborted = i.aborted + 1
	}
	i.mu.Unlock()
	return nil
}

type Fin struct {
	nb int
	mu *sync.Mutex
}

func (f *Fin) Finalize(e *Event) {
	f.mu.Lock()
	f.nb = f.nb + 1
	f.mu.Unlock()
}

func TestMassiveEvents(t *testing.T) {
	nbProducer := 50
	nbReceiver := 10
	producers := make(map[int]*Producer, nbProducer)
	fin := &Fin{mu: &sync.Mutex{}}
	es := NewEventSwitch(10).WithEventFinalizer(fin)

	for i := 0; i < nbProducer; i++ {
		p := &Producer{
			closed: make(chan struct{}),
			typ:    "a",
		}
		es.AddEmitter(p)
		producers[i] = p
	}
	receivers := make(map[int]*GlobalRec, nbReceiver)
	gr := &GlobalRec{
		typ: "a",
		mu:  &sync.Mutex{},
	}
	receivers[0] = gr
	es.AddReceiver(gr)
	in := &Inter{
		mu: &sync.Mutex{},
	}
	es.AddInterceptor(in)

	es.Start()

	for i := 0; i < nbProducer; i++ {
		go producers[i].EmitWhile()
	}

	time.Sleep(10 * time.Millisecond)
	for i := 0; i < nbProducer; i++ {
		producers[i].closed <- struct{}{}
	}
	es.Stop()
	time.Sleep(1 * time.Millisecond)
	total := 0
	for i := 0; i < nbProducer; i++ {
		total = total + producers[i].nbEmitted
	}
	if fin.nb != total {
		t.Fatal("finalize did not received all revents ", fin.nb, "-", total)
	}
	if in.received != total {
		t.Fatal("Multiproducers failed : ", gr.received, " - ", total)
	}
	if gr.received == total {
		t.Fatal("should have some aborted event")
	}
	if gr.received+in.aborted != total {
		t.Fatal("total must equal to received and aborted")
	}
}
