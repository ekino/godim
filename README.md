[![Build Status](https://travis-ci.org/ekino/godim.svg)](https://travis-ci.org/ekino/godim)
[![codecov](https://codecov.io/gh/ekino/godim/branch/master/graph/badge.svg)](https://codecov.io/gh/ekino/godim)
[![Go Report Card](https://goreportcard.com/badge/github.com/ekino/godim)](https://goreportcard.com/report/github.com/ekino/godim)

# godim - Go Dependency injection management 
## Version
v0.3 - Alpha Version - Everything may change

## Features
  * Lifecycle management
  * Simple Tag Declaration and injection
  * Configuration injection
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
  UserService *UserService `inject:"main.UserService"`
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
for now, only string and int64 are implemented

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
