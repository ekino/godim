[![Build Status](https://travis-ci.org/ekino/godim.svg)](https://travis-ci.org/ekino/godim)
[![codecov](https://codecov.io/gh/ekino/godim/branch/master/graph/badge.svg)](https://codecov.io/gh/ekino/godim)
[![Go Report Card](https://goreportcard.com/badge/github.com/ekino/godim)](https://goreportcard.com/report/github.com/ekino/godim)

# godim - Go Dependency injection management 
## Version
v0.5 - Alpha Version - Everything may change

## Features
  * Lifecycle management
  * Simple Tag Declaration and injection
  * Configuration injection
  * Event exchange throu the application
  
## Usage
Godim use tags to inject configuration and dependencies in a struct.
### Default Usage
A little example is sometimes better than a full explanation
````go
package main

import (
  "github.com/ekino/godim"
  "fmt"
)

type MyHandler struct {
  UserService *UserService `inject:"UserService"`
}

func (mh *MyHandler) doIt() {
  mh.UserService.doIt()
}

type UserService struct {
  MyKey string `config:"user.key"`
}

func (us *UserService) doIt() {
  fmt.Println("done :", us.MyKey)
}

func config(key string, kind reflect.Kind) (interface{}, error) {
  return "myuserKey"
}

func main(){
  mh := MyHandler{}
  us := UserService{}
  g := godim.NewGodim(godim.DefaultConfig().WithConfigurationFunction())
  g.DeclareDefault(&mh,&us)
  g.RunApp()
  
  mh.doIt()
  
}
````
will print
````
done :myuserKey
````

### Advanced usage

#### Name it

Implementing Identifier interface allows godim to know how you want to name your struct

````go
type Identifier interface {
	Key() string
}
````
By default, naming convention will use class definition.
For instance :

````go
package main

type UserService struct {

}
````
will have a name : main.UserService

#### Profile

You can define policies on how you want to enforce linking of your different layer.
For instance StrictHTTPProfile will define 3 kinds of layers:
- handler
- service
- repository

with a strict linking between them : repository can be injected in service, service can be injected in handler, all others possibilities are prohibited

#### AutoConfiguration

Providing to Godim a function like this one

````go
func configFunc (key string, value reflect.Value) (interface{}, error) {
...
}
````
will allow configuration parameters to be injected directly in your structs throu config tag
see [Godim-Viper](https://github.com/ekino/godim-viper) for an implementation of this function with Viper.

#### Specific initialization or closing

It is sometimes useful to initialize some things like connection to db during the life of the your app 
Two interfaces can be implemented for struct that needs specific 
````go
type Initializer interface {
	OnInit() error
}
type Closer interface {
	OnClose() error
}
`````

OnInit will be called after configuration and injection phases.
OnClose will be called when you close your app 
````go 
godim.CloseApp() 
````

### Lifecycle order

The current lifecycle order of godim will go through
- Declaration phase. use godim.Declare(...)
- Configuration phase, take all your config tags and fill them 
- Injection phase, take all your injection tags and link them
- Initialization phase, call all OnInit() func declared
- Running phase, your turn
- Closing phase, call all OnClose() func declared

#### Initialization priorization

You can handle the OnInit order if you need to, by implementing the following interface in your struct:
```go
type Prioritizer interface {
	Priority() int
}
```

Default priority is set to 0, by implementing this function you can say if you want to execute the OnInit method sooner (by returning a lower value) or later (with a higher value).

### Event Switch

Godim comes with a simple event switch that enable Event to be emitted from anywhere and received everywhere.
The definition of events Type throu the application is static, there is no dynamic declaration during the run lifecycle.

An Event looks like 

```go
type Event struct {
  Type string
  Payload map[string]interface{}
}
```

You need to consider events as immutable object (Read Only), as they can be accessed concurrently. But there is no protection against write in go yet

To emit an event your struct can implement EventEmitter and call Emit to send the event

```go
type MyStruct struct {
  EventEmitter
}
...
myStruct := &MyStruct{}
myStruct.Emit(&Event{Type:"a"})
```

There is 3 ways to observe those events, depending on the phase you want to interact with it.
After being emitted the event is buffered in the event switch. For each event, a go routine is launched, orchestrate a first level of interaction with EventInterceptor, interceptor have a priority, the lowest the first. During this phase an event can be aborted : it will stop propagation of the event on higher priority of interceptor or receiver. An aborted event will still go throu EventFinalizer if one is declared.
After orchestration phase, events are transmitted to each receiver in his own go routine in a choregraphic way.
When all events are managed by all interceptor and receivers, event finish his course in an EventFinalizer.

To intercept an event you need to implements EventInterceptor in your struct

```go
type EventInterceptor interface {
	Identifier
	Intercept(*Event) error
	InterceptPriority() int
}
```


To subscribe to an event you need to implements EventReceiver interface in your struct 

```go
type EventReceiver interface {
  Identifier
	ReceiveEvent(*Event)
	HandleEventTypes() []string
}
```

the HandleEventTypes method defines the subscribe events type by this receiver.
the ReceiveEvent will receive all events of type declared in the previous method.

EventFinalizer does not interact with event metadata. Typical usage is a final save in a db of the event.
EventFinalizer interface follow : 

```go
type EventFinalizer interface {
	Finalize(*Event)
}
```

There can be only one event finalizer in your application.
