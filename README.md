[![Build Status](https://travis-ci.org/ekino/godim.svg)](https://travis-ci.org/ekino/godim)

# godim - Go Dependency injection management 
## Version
v0.1 - Alpha Version - Everything may change

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

import "github.com/ekino/godim"

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

TODO ;-)
