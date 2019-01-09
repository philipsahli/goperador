package controller

import (
	"github.com/philipsahli/goperador/operator/pkg/controller/gontadorservice"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, gontadorservice.Add)
}
